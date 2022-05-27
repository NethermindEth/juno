package feeder

type Abi struct {
	Functions   []Function
	Events      []Event
	Structs     []Struct
	L1Handlers  []L1Handler
	Constructor *Constructor
}

type Function struct {
	FieldCommon
	Inputs  []Variable `json:"inputs"`
	Name    string     `json:"name"`
	Outputs []Variable `json:"outputs"`
}

type Event struct {
	FieldCommon
	Data []Variable `json:"data"`
	Keys []string   `json:"keys"`
	Name string     `json:"name"`
}

type Struct struct {
	FieldCommon
	Members []StructMember `json:"members"`
	Name    string         `json:"name"`
	Size    int64          `json:"size"`
}

type L1Handler struct {
	Function
}

type Constructor struct {
	Function
}

type StructMember struct {
	Variable
	Offset int64 `json:"offset"`
}

type Variable struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type FieldCommon struct {
	Type FieldType `json:"type"`
}

type FieldType string
