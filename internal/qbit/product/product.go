// Package product names the running product on the wire. The /health body of
// qbit-jay carries it so a client can tell qbit-jay from an Engram server that
// answers the same protocol (spec 6.3, port 7437).
package product

// ID is the value of the "product" field in the /health body. Engram does not
// send the field.
const ID = "qbit-jay"
