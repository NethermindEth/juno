package feeder

// represents ABI type
type Abi struct {
	Functions   []Function
	Events      []Event
	Structs     []Struct
	L1Handlers  []L1Handler
	Constructor *Constructor
}

// Represents Function abi
type Function struct {
	Inputs     []Variable `json:"inputs"`
	Name       string     `json:"name"`
	Outputs    []Variable `json:"outputs"`
	Mutability string     `json:"stateMutability"`
	FieldCommon
}

// Represents Event abi
type Event struct {
	FieldCommon
	Data []Variable `json:"data"`
	Keys []string   `json:"keys"`
	Name string     `json:"name"`
}

// Represents Struct abi
type Struct struct {
	FieldCommon
	Members []StructMember `json:"members"`
	Name    string         `json:"name"`
	Size    int64          `json:"size"`
}

// Represents L1Handler abi
type L1Handler struct {
	Function
}

// Represents Constructor abi
type Constructor struct {
	Function
}

// Represents StructMember abi
type StructMember struct {
	Variable
	Offset int64 `json:"offset"`
}

// Represents Variable abi
type Variable struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Represents FieldCommon; contains FieldType of abi object
type FieldCommon struct {
	Type FieldType `json:"type"`
}

type FieldType string
