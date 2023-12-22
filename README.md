# Blockchain task

## Goal

To track transactions happening between a certain contract and a certain account. Main task is to record down every transaction initiated by the account to the contract. 

## Requirements

1. Populate the environmental variables

```
POSTGRESQL_HOST = database host
POSTGRESQL_PASSWORD= database password
POSTGRESQL_PORT = database port (default is 5432)
POSTGRESQL_USER = database username (default is postgres)
POSTGRESQL_DBNAME = main database name
POSTGRESQL_CHECKUP = secondary database name
```

2. Install Golang (^1.21.5) and PostgreSQL (^16)

## How to test

- Individual testing
```
go test
```

For more information
```
go test -v
```

Specific function
```
go test -v -run <function name>
```