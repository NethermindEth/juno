// Package records holds the fast decoder's readers for Juno's stored types.
//
// A reader walks one record in a single pass, writing straight into the destination.
// Keys are matched by name, unknown keys are skipped and missing keys leave the field as it
// was, which is what fxamacker does. Readers only run on a zero destination (the wrapper
// sees to that), so pointers and slices are always allocated fresh.
//
// Everything a reader keeps is copied out of the input.
package records
