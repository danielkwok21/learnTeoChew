# learn teochew
https://teochew.danielkwok.com

# how to run locally
```shell
go run main.go
```

# how to access local sqlite db
```shell
sqlite3 db/learn_teochew.db

# show tables
.tables

# describe table
.schema <table_name>
```

# how to deploy
push to master

# how to upload audio folder from local to production
```shell
scp -r audio linode:~/learnTeochew
```