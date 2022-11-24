package table

import (
	"context"
	"fmt"
	"pouncecat/column"
	"pouncecat/source"
	"pouncecat/ui"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

var ctx = context.Background()

var Records []map[string]any

type Table struct {
	// The source name of the table.
	SrcName string
	// The output name of the table.
	DstName string
	// The columns of the table.
	Columns []*column.Column
	// Columns to index
	IndexCols []string
	// Ignore Foreign Key Errors
	IgnoreFKError bool
	// Ignore unique constraint errors
	IgnoreUniqueError bool
	// May not be on source?
	IgnoreMissing bool
}

func PrepareTables(pool *pgxpool.Pool) {
	pool.Exec(ctx, `DROP SCHEMA public CASCADE;
	CREATE SCHEMA public;
	GRANT ALL ON SCHEMA public TO postgres;
	GRANT ALL ON SCHEMA public TO public;
	COMMENT ON SCHEMA public IS 'standard public schema'`)

	pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")
}

// To ensure data is parsed before being inserted into the database, we use a temporary struct
type parsedDataStruct struct {
	SQL  string
	Args []any
}

func (t Table) Migrate(src source.Source, pool *pgxpool.Pool) {
	records, err := src.GetRecords(t.SrcName)

	Records = records // Just in case it is needed

	if err != nil {
		if t.IgnoreMissing {
			ui.NotifyMsg("info", "Table %s not found on source, skipping "+t.SrcName)
			records = []map[string]any{}
		} else {
			panic(err)
		}
	}

	bar := ui.StartBar(t.DstName, 2, true)

	cbar := ui.StartBar("collect info", int64(len(records)), false)

	_, err = pool.Exec(ctx, "DROP TABLE IF EXISTS "+t.DstName)

	if err != nil {
		panic(err)
	}

	// For the purposes of having a primary key
	_, err = pool.Exec(ctx, "CREATE TABLE "+t.DstName+" (itag UUID PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4())")

	if err != nil {
		panic(err)
	}

	// Create columns firstly
	for _, v := range t.Columns {
		_, err := pool.Exec(ctx, "ALTER TABLE "+t.DstName+" ADD COLUMN IF NOT EXISTS "+v.DstName+" "+v.SQLType()+" "+strings.Join(v.Meta(), " "))

		if err != nil {
			panic(err)
		}

		// Now add constraints
		for _, c := range v.Constraints.Raw() {
			_, err = pool.Exec(ctx, "ALTER TABLE "+t.DstName+" ADD CONSTRAINT "+t.DstName+"_"+v.DstName+"_"+c.Type+" "+c.SQL(v.DstName))

			if err != nil {
				fmt.Println("ALTER TABLE " + t.DstName + " ADD CONSTRAINT " + t.DstName + "_" + v.DstName + "_" + c.Type + " " + c.SQL(v.DstName))
				panic(err)
			}
		}
	}

	if len(t.IndexCols) > 0 {
		// Create index on these columns
		colList := strings.Join(t.IndexCols, ",")
		indexName := t.DstName + "_migindex"
		sqlStr := "CREATE INDEX " + indexName + " ON " + t.DstName + "(" + colList + ")"

		_, pgerr := pool.Exec(ctx, sqlStr)

		if pgerr != nil {
			panic(pgerr)
		}
	}

	var count int = 0

	var parsedData = []parsedDataStruct{}

	for _, record := range records {
		cbar.Increment()
		count++
		var args []any = []any{}
		var argQuotes []string = []string{}
		var colNames []string = []string{}

		var realI int
		for _, col := range t.Columns {
			arg := record[col.SrcName]

			for _, transform := range col.Transforms {
				arg = transform(record, arg)
			}

			extParsed, err := src.ExtParse(arg)

			if err == nil {
				arg = extParsed
			}

			if arg == "none" {
				arg = nil
			}

			var panicF bool

			if arg == "PANIC" {
				arg = nil
				panicF = true
			}

			if arg == nil {
				if col.Default != nil {
					arg = col.Default
				}

				if col.SQLDefault != "" && arg == nil {
					arg = col.SQLDefault
				}

				if arg == "NULL" {
					arg = nil
				}

				if arg == "uuid_generate_v4()" {
					continue
				}

				if arg == "SKIP" {
					ui.NotifyMsg("warning", "Skipping row due to default value at iteration "+strconv.Itoa(count))
					break
				} else if panicF {
					panic("Panic due to default value at iteration " + strconv.Itoa(count) + " on column " + col.SrcName)
				}
			}

			args = append(args, arg)
			argQuotes = append(argQuotes, "$"+strconv.Itoa(realI+1))
			realI++
			colNames = append(colNames, col.DstName)
		}

		parsedData = append(parsedData, parsedDataStruct{
			SQL:  "INSERT INTO " + t.DstName + " (" + strings.Join(colNames, ",") + ") VALUES (" + strings.Join(argQuotes, ",") + ")",
			Args: args,
		})
	}

	bar.Increment()

	pbar := ui.StartBar("inserting data", int64(len(parsedData)), false)

	for i, data := range parsedData {
		pbar.Increment()

		_, err := pool.Exec(ctx, data.SQL, data.Args...)

		if err != nil {
			if t.IgnoreFKError && strings.Contains(err.Error(), "violates foreign key") {
				ui.NotifyMsg("warning", "Ignoring foreign key error on iter "+strconv.Itoa(i)+": "+err.Error())
				continue
			} else if t.IgnoreUniqueError && strings.Contains(err.Error(), "unique constraint") {
				ui.NotifyMsg("warning", "Ignoring unique error on iter "+strconv.Itoa(i)+": "+err.Error())
				continue
			}

			ui.NotifyMsg("error", "Error on iter "+strconv.Itoa(i)+": "+err.Error())

			panic(err.Error() + ":" + data.SQL)
		}
	}

	bar.Increment()

	cbar.Abort(true)
	pbar.Abort(true)
	bar.Abort(true)

	bar.Wait()

	time.Sleep(1 * time.Second)
}
