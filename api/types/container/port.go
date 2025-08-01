// Code generated by go-swagger; DO NOT EDIT.

package container

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

// Port An open port on a container
// Example: {"PrivatePort":8080,"PublicPort":80,"Type":"tcp"}
//
// swagger:model Port
type Port struct {

	// Host IP address that the container's port is mapped to
	IP string `json:"IP,omitempty"`

	// Port on the container
	// Required: true
	PrivatePort uint16 `json:"PrivatePort"`

	// Port exposed on the host
	PublicPort uint16 `json:"PublicPort,omitempty"`

	// type
	// Required: true
	// Enum: ["tcp","udp","sctp"]
	Type string `json:"Type"`
}
