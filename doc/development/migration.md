# Migration

### Adding new migration

1. The project used rockerhopper for db migration.
   https://github.com/c9s/rockhopper


2. Create migration files


You can use the util script to generate the migration files:

```
bash utils/generate-new-migration.sh add_pnl_column
```

Or, you can generate the migration files separately:

```sh
rockhopper --config rockhopper_sqlite.yaml create --type sql add_pnl_column
rockhopper --config rockhopper_mysql.yaml create --type sql add_pnl_column
```


Be sure to edit both sqlite3 and mysql migration files. ( [Sample](.../../migrations/mysql/20210531234123_add_kline_taker_buy_columns.sql) )

To test the drivers, you have to update the rockhopper_mysql.yaml file to connect your database,
then do:

```sh
rockhopper --config rockhopper_sqlite.yaml up
rockhopper --config rockhopper_mysql.yaml up
```

Then run the following command to compile the migration files into go files:

```shell
make migrations
```

or

```shell
rockhopper compile --config rockhopper_mysql.yaml --output pkg/migrations/mysql
rockhopper compile --config rockhopper_sqlite.yaml --output pkg/migrations/sqlite3
git add -v pkg/migrations && git commit -m "compile and update migration package" pkg/migrations || true
```


If you want to override the DSN and the Driver defined in the YAML config file, you can add some env vars in your dotenv file like this:

```shell
ROCKHOPPER_DRIVER=mysql
ROCKHOPPER_DIALECT=mysql
ROCKHOPPER_DSN="root:123123@unix(/opt/local/var/run/mysql57/mysqld.sock)/bbgo"
```

And then, run:

```shell
dotenv -f .env.local -- rockhopper --config rockhopper_mysql.yaml up
```
