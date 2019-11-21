# RestQL

A GraphQL API that serves its data through REST as well.

## Starting the server

First run `dep ensure` to make sure your dependencies are up to date. Then just `go run main.go`.

Once it's running, you should be able to point [GraphiQL](https://github.com/skevy/graphiql-app) at
http://localhost:8080/graphql and use http://localhost:8080/<whatever> for all REST requests.

## How does it work?

Take a look at [schema3.graphql](schema3.graphql). This uses GraphQL's schema language to define the schema of our API.
Notice the extra annotations (or "directives") on some of the objects and fields: @rest, @path, and @restBase.

### @rest

The @rest directive tells RestQL to generate a REST route for that field. For example, say we have the following simple
schema:

```graphql
schema {
    query: Query
}

type Query {
    account(id: ID!): Account! @rest
}
```

Because the `account` field has the @rest directive, you can access that data via REST in addition to GraphQL. The URL
will simply be `/account?id=<your id>`. Any arguments to the fields will be GET parameters in the REST version.

### @path

The @path directive simply puts a parameter in the URL instead of the query string. For example, say we revise our
previous schema:

```graphql
schema {
    query: Query
}

type Query {
    account(id: ID! @path): Account! @rest
}
```

Now the URL for the REST endpoint will be `/account/:id`. You may have multiple @path parameters, and they will appear
in the order they appear within the GraphQL schema. For example, this schema:

```graphql
schema {
    query: ExampleQuery
}

type ExampleQuery {
    field(paramOne: String! @path, paramTwo: String! @path): Example! @rest
}
```

will result in this REST endpoint:

```
/field/:paramOne/:paramTwo
```

### @restBase(type: String!, query: String!)

The @restBase directive enables REST routes to be nested underneath each other. For example, say we want to be able to
get a user's shifts. Our schema might look like this:

```graphql
schema {
    query: Query
}

type Query {
    user(id: ID! @path): User! @rest
}

type User @restBase(type: "Query", query: "user") {
    shifts: Shift! @rest
}
```

The @restBase directive on `User` tells RestQL that a `User` can canonically be accessed using the `user` field on the
`Query` object. RestQL will thus place all @rest routes from `User` underneath `/user/:id`. So this example schema will
generate the following REST routes:

```
/user/:id
/user/:id/shifts
```

While this does not allow the REST version of the API to arbitrarily traverse the graph like GraphQL, it does allow
every field of every type to be accessible via REST.

## Some examples

Try the following GraphQL query:

```graphql
{
  example1_SimpleAccount: account(id: "123") {
    id
    users {
      id
      type
    }
  }
  example2_EnumQuery: account(id: "123") {
    usersByType(type: ACCOUNT_HOLDER) {
      id
      type
    }
  }
}
```

Take note of the data it returns. Then try the following REST routes:

```
/account/123
/account/123/users
/account/123/usersByType?type=ACCOUNT_HOLDER
```
