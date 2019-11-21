package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/bvisness/restql/api"
	"github.com/bvisness/restql/schema"
	"github.com/bvisness/restql/testdata"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
)

/* TODO:
- Warn or error on duplicate parameter names in a chain
- Validate missing parameters in REST stuff (and maybe other validations too?)
- Hook it up to our real API

Real-world problems to solve after the concept is proven:
- Pagination? (don't wanna but we probably have to)
- Rate limiting?
- Think about documentation, and see if any extra directives or comment-parsing would be useful.
*/

func main() {
	scma, restRoutes := LoadSchemaFile("schema.graphql", schema.ObjectFieldResolvers{
		"Query": schema.FieldResolvers{
			"person": func(p graphql.ResolveParams) (interface{}, error) {
				if person, ok := testdata.People[p.Args["id"].(string)]; ok {
					return person, nil
				} else {
					return nil, api.NewErrorWithRestStatus(
						http.StatusNotFound,
						fmt.Errorf("no person found with ID '%s'", p.Args["id"]),
					)
				}
			},
			"account": func(p graphql.ResolveParams) (interface{}, error) {
				if account, ok := testdata.Accounts[p.Args["id"].(string)]; ok {
					return account, nil
				} else {
					return nil, api.NewErrorWithRestStatus(
						http.StatusNotFound,
						fmt.Errorf("no account found with ID '%s'", p.Args["id"]),
					)
				}
			},
			"user": func(p graphql.ResolveParams) (interface{}, error) {
				if user, ok := testdata.Users[p.Args["id"].(string)]; ok {
					return user, nil
				} else {
					return nil, api.NewErrorWithRestStatus(
						http.StatusNotFound,
						fmt.Errorf("no user found with ID '%s'", p.Args["id"]),
					)
				}
			},
		},
		"Person": schema.FieldResolvers{
			// TODO: Need some way to easily return plain old JSON fields
			"id": func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(testdata.Person).ID, nil
			},
			"users": func(p graphql.ResolveParams) (interface{}, error) {
				var users []testdata.User
				for _, user := range testdata.Users {
					if user.PersonID == p.Source.(testdata.Person).ID {
						users = append(users, user)
					}
				}

				return users, nil
			},
		},
		"Account": schema.FieldResolvers{
			"id": func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(testdata.Account).ID, nil
			},
			"plan": func(p graphql.ResolveParams) (interface{}, error) {
				id := p.Source.(testdata.Account).PlanID
				if plan, ok := testdata.Plans[id]; ok {
					return plan, nil
				} else {
					return nil, fmt.Errorf("no plan found with ID '%s'", id)
				}
			},
			"users": func(p graphql.ResolveParams) (interface{}, error) {
				var users []testdata.User
				for _, user := range testdata.Users {
					if user.AccountID == p.Source.(testdata.Account).ID {
						users = append(users, user)
					}
				}

				return users, nil
			},
			"usersByType": func(p graphql.ResolveParams) (interface{}, error) {
				users := []testdata.User{}
				for _, user := range testdata.Users {
					if user.AccountID == p.Source.(testdata.Account).ID && user.Type == p.Args["type"] {
						users = append(users, user)
					}
				}

				return users, nil
			},
		},
		"Plan": schema.FieldResolvers{
			"id": func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(testdata.Plan).ID, nil
			},
			"name": func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(testdata.Plan).Name, nil
			},
		},
		"User": schema.FieldResolvers{
			"id": func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(testdata.User).ID, nil
			},
			"person": func(p graphql.ResolveParams) (interface{}, error) {
				id := p.Source.(testdata.User).PersonID
				if person, ok := testdata.People[id]; ok {
					return person, nil
				} else {
					return nil, fmt.Errorf("no person found with ID '%s'", id)
				}
			},
			"account": func(p graphql.ResolveParams) (interface{}, error) {
				id := p.Source.(testdata.User).AccountID
				if account, ok := testdata.Accounts[id]; ok {
					return account, nil
				} else {
					return nil, fmt.Errorf("no account found with ID '%s'", id)
				}
			},
			"type": func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(testdata.User).Type, nil
			},
		},
	})

	server := gin.Default()
	server.Use(
		cors.New(cors.Config{
			AllowAllOrigins: true,
			AllowMethods:    []string{"POST", "OPTIONS"},
			AllowHeaders: []string{
				"Origin",
				"Authorization",
				"X-Requested-With",
				"Content-Type",
				"Accept",
				"W-Token",
				"W-UserId",
			},
		}),
		api.ErrorMiddleware(),
	)
	server.OPTIONS("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello!")
	})
	server.GET("/graphql", GinHandlerForGraphQLSchema(scma))
	for _, route := range restRoutes {
		server.GET(route.Route, route.Handler)
	}

	log.Fatal(server.Run())
}

func LoadSchemaFile(filename string, resolvers schema.ObjectFieldResolvers) (*graphql.Schema, []schema.RestRouteInfo) {
	schemaStr, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	scma, restRoutes := schema.MustParseSchema(string(schemaStr), resolvers)

	return scma, restRoutes
}

func GinHandlerForGraphQLSchema(scma *graphql.Schema) gin.HandlerFunc {
	graphqlHandler := handler.New(&handler.Config{
		Schema: scma,
		Pretty: false,
	})

	return func(c *gin.Context) {
		graphqlHandler.ServeHTTP(c.Writer, c.Request)
	}
}
