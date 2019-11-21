package schema

import (
	"fmt"
	"log"
	"net/http"
	"reflect"

	"github.com/bvisness/restql/api"
	"github.com/gin-gonic/gin"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
)

func check(err error, msg string) {
	if err != nil {
		panic(fmt.Errorf("%s: %s", msg, err))
	}
}

type SchemaDefinition struct {
	QueryTypeName    string
	MutationTypeName string
}

type RestTreeNode struct {
	ObjectDefinition *ast.ObjectDefinition
	FieldDefinition  *ast.FieldDefinition
	Children         []*RestTreeNode
}

type RestRouteInfo struct {
	Route   string
	Handler gin.HandlerFunc
}

type SchemaHelper struct {
	SchemaDefinition  SchemaDefinition
	ObjectDefinitions map[string]*ast.ObjectDefinition
	ScalarDefinitions map[string]*ast.ScalarDefinition
	EnumDefinitions   map[string]*ast.EnumDefinition
	FieldDefinitions  map[string]map[string]*ast.FieldDefinition
	// TODO: enums and stuff
}

type ObjectFieldResolvers map[string]FieldResolvers
type FieldResolvers map[string]graphql.FieldResolveFn

func MustParseSchema(schemaStr string, resolvers ObjectFieldResolvers) (*graphql.Schema, []RestRouteInfo) {
	var restRoutes []RestRouteInfo

	doc, err := parser.Parse(
		parser.ParseParams{
			Source: schemaStr,
			Options: parser.ParseOptions{
				NoLocation: true,
				NoSource:   true,
			},
		},
	)
	check(err, "failed to parse schema document")

	allTypes := map[string]graphql.Type{}
	schemaHelper := GetSchemaHelper(doc)

	// Pass 1: Loop through all types and generate objects for them. Don't put any fields on objects,
	// though, because we need to know about all types in the schema before we can do that.

	for _, objDef := range schemaHelper.ObjectDefinitions {
		allTypes[objDef.Name.Value] = graphql.NewObject(graphql.ObjectConfig{
			Name:   objDef.Name.Value,
			Fields: graphql.Fields{},
		})
	}

	for _, scalarDef := range schemaHelper.ScalarDefinitions {
		allTypes[scalarDef.Name.Value] = graphql.NewScalar(graphql.ScalarConfig{
			Name: scalarDef.Name.Value,
			Serialize: func(value interface{}) interface{} {
				return "datetime!!" // TODO: obviously bad
			},
			// TODO: grab the parsing and serializing functions
		})
	}

	for _, enumDef := range schemaHelper.EnumDefinitions {
		values := make(graphql.EnumValueConfigMap)

		for _, val := range enumDef.Values {
			values[val.Name.Value] = &graphql.EnumValueConfig{
				Value: val.Name.Value,
			}
		}

		allTypes[enumDef.Name.Value] = graphql.NewEnum(graphql.EnumConfig{
			Name:   enumDef.Name.Value,
			Values: values,
		})
	}

	// TODO: Enum definitions? Other types of definitions?

	// Pass 2: Loop through all the *objects* and put fields on them, now that we know about all the types.

	for _, objDef := range schemaHelper.ObjectDefinitions {
		obj := allTypes[objDef.Name.Value].(*graphql.Object)
		for _, fieldDef := range objDef.Fields {
			args := graphql.FieldConfigArgument{}
			for _, inputValueDef := range fieldDef.Arguments {
				args[inputValueDef.Name.Value] = &graphql.ArgumentConfig{
					Type:         GraphqlGoFieldType(inputValueDef.Type, allTypes),
					DefaultValue: GetDefaultValue(inputValueDef),
				}
			}

			if resolvers[objDef.Name.Value] == nil || resolvers[objDef.Name.Value][fieldDef.Name.Value] == nil {
				panic(fmt.Errorf("no resolver found for %s/%s", objDef.Name.Value, fieldDef.Name.Value))
			}

			obj.AddFieldConfig(fieldDef.Name.Value, &graphql.Field{
				Name:    fieldDef.Name.Value,
				Type:    GraphqlGoFieldType(fieldDef.Type, allTypes),
				Args:    args,
				Resolve: resolvers[objDef.Name.Value][fieldDef.Name.Value],
			})
		}
	}

	// Pass 3: Build and traverse the tree of REST routes, creating the actual Gin routes and functions.

	if schemaHelper.SchemaDefinition.QueryTypeName != "" {
		nodes := map[*ast.FieldDefinition]*RestTreeNode{}

		// Make a node for each field, save them in a map by field definition
		for _, objDef := range schemaHelper.ObjectDefinitions {
			for _, fieldDef := range objDef.Fields {
				if restDirective := GetFieldDirective(fieldDef, "rest"); restDirective != nil {
					newNode := &RestTreeNode{
						ObjectDefinition: objDef,
						FieldDefinition:  fieldDef,
					}
					nodes[fieldDef] = newNode
				}
			}
		}

		// Loop through again, linking them together according to restBase
		for _, node := range nodes {
			restBase := GetObjectDirective(node.ObjectDefinition, "restBase")

			if restBase != nil {
				typeName := GetDirectiveArgument(restBase, "type").(string)
				queryName := GetDirectiveArgument(restBase, "query").(string)

				if parentField, fieldExists := schemaHelper.FieldDefinitions[typeName][queryName]; fieldExists {
					if GetFieldDirective(parentField, "rest") == nil {
						check(
							fmt.Errorf("field '%s' on type '%s' does not have the @rest directive", queryName, typeName),
							fmt.Sprintf("could not follow @restBase on type '%s'", node.ObjectDefinition.Name.Value),
						)
					}

					parentNode := nodes[parentField]
					parentNode.Children = append(parentNode.Children, node)
				} else {
					check(
						fmt.Errorf("field '%s' does not exist on type '%s'", queryName, typeName),
						fmt.Sprintf("could not follow @restBase on type '%s'", node.ObjectDefinition.Name.Value),
					)
				}
			}
		}

		// Link up the base Query fields to a root node with a nil field
		restRoot := &RestTreeNode{}
		rootQueryObject := schemaHelper.ObjectDefinitions[schemaHelper.SchemaDefinition.QueryTypeName]
		for _, field := range rootQueryObject.Fields {
			node := nodes[field]
			restRoot.Children = append(restRoot.Children, node)
		}

		// Traverse the tree, building routes
		restRoutes = BuildRestRoutes(restRoot, resolvers)

		/*
			TODO: STREAM OF CONSCIOUSNESS
			Gotta build a tree! Recursively following a tree of routes lets you pass the complete resolver function
			from the parent to the child, so the child can call it to get the source. I guess you'll probably have to
			make up some resolver params that work, but they only have to work within the REST stuff.

			You can also build the actual route string in a much more natural way, so that's neat.

			Uh oh. Route parameters might make this a little difficult.

			Uh oh! GET parameters might ruin everything!

			Think through some cases.

			Maybe you can prefix parameter names with the parent resource's name? How to handle /foo/:id/bar/:id on
			the resolving end?
		*/

	}

	// Done processing the schema. All the objects are created and ready. Time to put them in a schema!

	var schemaConfig graphql.SchemaConfig
	if schemaHelper.SchemaDefinition.QueryTypeName != "" {
		schemaConfig.Query = allTypes[schemaHelper.SchemaDefinition.QueryTypeName].(*graphql.Object)
	}
	for _, tipe := range allTypes {
		schemaConfig.Types = append(schemaConfig.Types, tipe)
	}
	schema, err := graphql.NewSchema(schemaConfig)
	check(err, "failed to generate schema")

	return &schema, restRoutes
}

func BuildRestRoutes(root *RestTreeNode, resolvers ObjectFieldResolvers) []RestRouteInfo {
	return BuildRestRoutesRecursive(root, "", nil, resolvers)
}
func BuildRestRoutesRecursive(node *RestTreeNode, parentRoute string, parentNodes []*RestTreeNode, resolvers ObjectFieldResolvers) []RestRouteInfo {
	// Skip the root node
	if node.FieldDefinition == nil {
		var restRoutes []RestRouteInfo
		for _, child := range node.Children {
			restRoutes = append(restRoutes, BuildRestRoutesRecursive(child, "", nil, resolvers)...)
		}
		return restRoutes
	}

	// Build the route string
	newRoute := fmt.Sprintf("%s/%s", parentRoute, node.FieldDefinition.Name.Value)
	for _, arg := range node.FieldDefinition.Arguments { // add any path arguments
		if GetInputValueDirective(arg, "path") != nil {
			newRoute = fmt.Sprintf("%s/:%s", newRoute, arg.Name.Value)
		}
	}

	allNodes := append(parentNodes, node)

	routeHandler := func(c *gin.Context) {
		var resolvedValue interface{} = nil
		for _, currentNode := range allNodes {
			args := map[string]interface{}{}

			for _, arg := range currentNode.FieldDefinition.Arguments {
				name := arg.Name.Value

				if GetInputValueDirective(arg, "path") != nil {
					args[name] = c.Param(name)
				} else {
					args[name] = c.Query(name)
				}
			}

			resolver := resolvers[currentNode.ObjectDefinition.Name.Value][currentNode.FieldDefinition.Name.Value]

			var err error
			resolvedValue, err = resolver(graphql.ResolveParams{
				Source: resolvedValue,
				Args:   args,
			})
			if err != nil {
				if restErr, ok := err.(api.ErrorWithRestStatus); ok {
					c.Status(restErr.Status)
					err = restErr.GetWrappedError()
				} else {
					c.Status(http.StatusInternalServerError)
				}

				log.Print(reflect.TypeOf(err))
				log.Printf("%+v", err)

				c.Error(&gin.Error{
					Err:  err,
					Type: gin.ErrorTypePublic,
				})

				c.Abort()
				return
			}
		}

		c.JSON(http.StatusOK, api.Response{
			Data: resolvedValue,
		})
	}

	restRoutes := []RestRouteInfo{
		{
			Route:   newRoute,
			Handler: routeHandler,
		},
	}
	newParentNodes := append(parentNodes, node)
	for _, child := range node.Children {
		restRoutes = append(restRoutes, BuildRestRoutesRecursive(child, newRoute, newParentNodes, resolvers)...)
	}
	return restRoutes
}

func GetSchemaHelper(doc *ast.Document) SchemaHelper {
	result := SchemaHelper{
		SchemaDefinition:  SchemaDefinition{},
		ObjectDefinitions: make(map[string]*ast.ObjectDefinition),
		FieldDefinitions:  make(map[string]map[string]*ast.FieldDefinition),
		ScalarDefinitions: make(map[string]*ast.ScalarDefinition),
		EnumDefinitions:   make(map[string]*ast.EnumDefinition),
	}

	for _, node := range doc.Definitions {
		switch n := node.(type) {
		case *ast.SchemaDefinition:
			for _, opType := range n.OperationTypes {
				if opType.Operation == "query" {
					result.SchemaDefinition.QueryTypeName = opType.Type.Name.Value
				}
				if opType.Operation == "mutation" {
					result.SchemaDefinition.MutationTypeName = opType.Type.Name.Value
				}
			}
		case *ast.ObjectDefinition:
			result.ObjectDefinitions[n.Name.Value] = n

			result.FieldDefinitions[n.Name.Value] = map[string]*ast.FieldDefinition{}
			for _, field := range n.Fields {
				result.FieldDefinitions[n.Name.Value][field.Name.Value] = field
			}
		case *ast.ScalarDefinition:
			result.ScalarDefinitions[n.Name.Value] = n
		case *ast.EnumDefinition:
			result.EnumDefinitions[n.Name.Value] = n
		}
	}

	return result
}

func GraphqlGoFieldType(tipe ast.Type, types map[string]graphql.Type) graphql.Type {
	switch t := tipe.(type) {
	case *ast.NonNull:
		return graphql.NewNonNull(GraphqlGoFieldType(t.Type, types))
	case *ast.List:
		return graphql.NewList(GraphqlGoFieldType(t.Type, types))
	case *ast.Named:
		switch name := t.Name.Value; name {
		case "Int":
			return graphql.Int
		case "Float":
			return graphql.Float
		case "String":
			return graphql.String
		case "Boolean":
			return graphql.Boolean
		case "ID":
			return graphql.ID
		default:
			return types[name]
		}
	}

	return nil
}

func GetDefaultValue(def *ast.InputValueDefinition) interface{} {
	if def.DefaultValue == nil {
		return nil
	}

	return def.DefaultValue.GetValue()
}

func GetDirectiveByName(dirs []*ast.Directive, name string) *ast.Directive {
	for _, directive := range dirs {
		if directive.Name.Value == name {
			return directive
		}
	}

	return nil
}
func GetObjectDirective(def *ast.ObjectDefinition, name string) *ast.Directive {
	return GetDirectiveByName(def.Directives, name)
}
func GetFieldDirective(def *ast.FieldDefinition, name string) *ast.Directive {
	return GetDirectiveByName(def.Directives, name)
}
func GetInputValueDirective(def *ast.InputValueDefinition, name string) *ast.Directive {
	return GetDirectiveByName(def.Directives, name)
}

func GetDirectiveArgument(def *ast.Directive, name string) interface{} {
	for _, arg := range def.Arguments {
		if arg.Name.Value == name {
			return arg.Value.GetValue()
		}
	}

	return nil
}
