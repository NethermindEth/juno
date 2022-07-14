// Package db provides a simple interface to a database. The base behavior is described by the DatabaseOperations,
// Database and DatabaseTransactional interfaces, any database implementation that match with these interfaces can be
// used as a Juno database.
//
// Juno uses MDBX as its current database implementation. MDBX has a global environment (the database file) and within
// it a group of isolated named databases. The environment is initialized with the InitializeMDBXEnv function, then
// databases can be created with the NewMDBXDatabase function, an MDBXDatabse is a named database within the
// initialized environment. If the database already exists, the database is opened, otherwise, it is created.
package db
