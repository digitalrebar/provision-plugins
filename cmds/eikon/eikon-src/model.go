package main

// Validator is an interface that enables an object to validated.
type Validator interface {
	Validate() error
}

// Result is a data type
type Result int

const (
	// ResultSuccess indicates an action is successful
	ResultSuccess Result = iota
	// ResultRescan indicates an action needs to have the system rescan
	ResultRescan
	// ResultFailed indicates an action has failed.
	ResultFailed
)
