// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package schema

import (
	"context"
	"testing"

	"github.com/facebookincubator/ent/schema/field"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/facebookincubator/ent/dialect/sql"
	"github.com/stretchr/testify/require"
)

func TestPostgres_Create(t *testing.T) {
	tests := []struct {
		name    string
		tables  []*Table
		options []MigrateOption
		before  func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "tx failed",
			before: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(sqlmock.ErrCancelled)
			},
			wantErr: true,
		},
		{
			name: "no tables",
			before: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(escape("SHOW server_version_num")).
					WillReturnRows(sqlmock.NewRows([]string{"server_version_num"}).AddRow("120000"))
				mock.ExpectCommit()
			},
		},
		{
			name: "create new table",
			tables: []*Table{
				{
					Name: "users",
					PrimaryKey: []*Column{
						{Name: "id", Type: field.TypeInt, Increment: true},
					},
					Columns: []*Column{
						{Name: "id", Type: field.TypeInt, Increment: true},
						{Name: "name", Type: field.TypeString, Nullable: true},
						{Name: "age", Type: field.TypeInt},
						{Name: "doc", Type: field.TypeJSON, Nullable: true},
						{Name: "enums", Type: field.TypeEnum, Enums: []string{"a", "b"}},
					},
				},
			},
			before: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(escape("SHOW server_version_num")).
					WillReturnRows(sqlmock.NewRows([]string{"server_version_num"}).AddRow("120000"))
				mock.ExpectQuery(escape(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE "table_schema" = CURRENT_SCHEMA() AND "table_name" = $1`)).
					WithArgs("users").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectExec(escape(`CREATE TABLE IF NOT EXISTS "users"("id" bigint GENERATED BY DEFAULT AS IDENTITY NOT NULL, "name" varchar NULL, "age" bigint NOT NULL, "doc" jsonb NULL, "enums" varchar NOT NULL, PRIMARY KEY("id"))`)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
		},
		{
			name: "create new table with foreign key",
			tables: func() []*Table {
				var (
					c1 = []*Column{
						{Name: "id", Type: field.TypeInt, Increment: true},
						{Name: "name", Type: field.TypeString, Nullable: true},
						{Name: "created_at", Type: field.TypeTime},
					}
					c2 = []*Column{
						{Name: "id", Type: field.TypeInt, Increment: true},
						{Name: "name", Type: field.TypeString},
						{Name: "owner_id", Type: field.TypeInt, Nullable: true},
					}
					t1 = &Table{
						Name:       "users",
						Columns:    c1,
						PrimaryKey: c1[0:1],
					}
					t2 = &Table{
						Name:       "pets",
						Columns:    c2,
						PrimaryKey: c2[0:1],
						ForeignKeys: []*ForeignKey{
							{
								Symbol:     "pets_owner",
								Columns:    c2[2:],
								RefTable:   t1,
								RefColumns: c1[0:1],
								OnDelete:   Cascade,
							},
						},
					}
				)
				return []*Table{t1, t2}
			}(),
			before: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(escape("SHOW server_version_num")).
					WillReturnRows(sqlmock.NewRows([]string{"server_version_num"}).AddRow("120000"))
				mock.ExpectQuery(escape(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE "table_schema" = CURRENT_SCHEMA() AND "table_name" = $1`)).
					WithArgs("users").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectExec(escape(`CREATE TABLE IF NOT EXISTS "users"("id" bigint GENERATED BY DEFAULT AS IDENTITY NOT NULL, "name" varchar NULL, "created_at" timestamp with time zone NOT NULL, PRIMARY KEY("id"))`)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(escape(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE "table_schema" = CURRENT_SCHEMA() AND "table_name" = $1`)).
					WithArgs("pets").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectExec(escape(`CREATE TABLE IF NOT EXISTS "pets"("id" bigint GENERATED BY DEFAULT AS IDENTITY NOT NULL, "name" varchar NOT NULL, "owner_id" bigint NULL, PRIMARY KEY("id"))`)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(escape(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS WHERE "table_schema" = CURRENT_SCHEMA() AND "constraint_type" = $1 AND "constraint_name" = $2`)).
					WithArgs("FOREIGN KEY", "pets_owner").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectExec(escape(`ALTER TABLE "pets" ADD CONSTRAINT "pets_owner" FOREIGN KEY("owner_id") REFERENCES "users"("id") ON DELETE CASCADE`)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			tt.before(mock)
			migrate, err := NewMigrate(sql.OpenDB("postgres", db), tt.options...)
			require.NoError(t, err)
			err = migrate.Create(context.Background(), tt.tables...)
			require.Equal(t, tt.wantErr, err != nil, err)
		})
	}
}