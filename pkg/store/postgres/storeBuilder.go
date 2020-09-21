//Copyright (c) 2018, Oracle and/or its affiliates. All rights reserved.
//Licensed under the Universal Permissive License (UPL) Version 1.0 as shown at http://oss.oracle.com/licenses/upl.
package postgres

import (
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/teramoby/speedle-plus/api/pms"
	"github.com/teramoby/speedle-plus/pkg/store"
)

const (
	StoreType = "postgres"

	//Following are keys of postgres store properties
	PostgresURIKey          = "PostgresURI"
	PostgresTablePrefixKey  = "PostgresTablePrefix"
	PostgresMaxOpenConnsKey = "PostgresMaxOpenConns"
	PostgresMaxIdleConnsKey = "PostgresMaxIdleConns"

	PostgresURIFlag          = "postgres_uri"
	PostgresTablePrefixFlag  = "postgres_table_prefix"
	PostgresMaxOpenConnsFlag = "postgres_max_open_conns"
	PostgresMaxIdleConnsFlag = "postgres_max_idle_conns"

	//default property values
	DefaultURI          = "postgres://localhost:5432/tinateams?sslmode=disable"
	DefaultTablePrefix  = "speedle_"
	DefaultMaxOpenConns = 10
	DefaultMaxIdleConns = 10
)

type PostgresStoreBuilder struct{}

func (msb PostgresStoreBuilder) NewStore(config map[string]interface{}) (pms.PolicyStoreManager, error) {
	postgresURI := config[PostgresURIKey].(string)
	db, err := sqlx.Connect("postgres", postgresURI)
	if err != nil {
		log.Fatal(err)
	}

	if postgresMaxOpenConns, ok := config[PostgresMaxOpenConnsKey].(string); ok {
		conns, err := strconv.Atoi(postgresMaxOpenConns)
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(conns)
	}

	if postgresMaxIdleConns, ok := config[PostgresMaxIdleConnsKey].(string); ok {
		conns, err := strconv.Atoi(postgresMaxIdleConns)
		if err != nil {
			return nil, err
		}
		db.SetMaxIdleConns(conns)
	}

	db.SetConnMaxLifetime(15 * time.Minute)

	postgresTablePrefix := config[PostgresTablePrefixKey].(string)
	return &Store{client: db, tablePrefix: postgresTablePrefix}, nil
}

func (msb PostgresStoreBuilder) GetStoreParams() map[string]string {
	return map[string]string{
		PostgresURIFlag:          PostgresURIKey,
		PostgresTablePrefixFlag:  PostgresTablePrefixKey,
		PostgresMaxOpenConnsFlag: PostgresMaxOpenConnsKey,
		PostgresMaxIdleConnsFlag: PostgresMaxIdleConnsKey,
	}
}

func init() {
	pflag.String(PostgresURIFlag, DefaultURI, "Store config: URI of postgres DB.")
	pflag.String(PostgresTablePrefixFlag, DefaultTablePrefix, "Store config: postgres table prefix")
	pflag.Int(PostgresMaxOpenConnsFlag, DefaultMaxOpenConns, "Store config: max open connections")
	pflag.Int(PostgresMaxIdleConnsFlag, DefaultMaxIdleConns, "Store config: max idle connections")

	store.Register(StoreType, PostgresStoreBuilder{})
}
