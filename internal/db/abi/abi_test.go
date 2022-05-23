package abi

var abis = map[string]Abi{
	"1bd7ca87f139693e6681be2042194cf631c4e8d77027bf0ea9e6d55fc6018ac": {
		Functions: []*Function{
			{
				Name: "initialize",
				Inputs: []*Function_Input{
					{
						Name: "signer",
						Type: "felt",
					},
					{
						Name: "guardian",
						Type: "felt",
					},
				},
				Outputs: nil,
			},
			{
				Name: "__execute__",
				Inputs: []*Function_Input{
					{
						Name: "call_array_len",
						Type: "felt",
					},
					{
						Name: "call_array",
						Type: "felt",
					},
					{
						Name: "calldata_len",
						Type: "felt",
					},
					{
						Name: "calldata",
						Type: "felt*",
					},
					{
						Name: "nonce",
						Type: "felt",
					},
				},
				Outputs: []*Function_Output{
					{
						Name: "retdata_size",
						Type: "felt",
					},
					{
						Name: "retdata",
						Type: "felt*",
					},
				},
			},
			{
				Name: "upgrade",
				Inputs: []*Function_Input{
					{
						Name: "implementation",
						Type: "felt",
					},
				},
				Outputs: nil,
			},
		},
		Events: []*Event{
			{
				Data: []*Event_Data{
					{
						Name: "new_signer",
						Type: "felt",
					},
				},
				Keys: nil,
				Name: "signer_changed",
			},
			{
				Data: []*Event_Data{
					{
						Name: "new_guardian",
						Type: "felt",
					},
				},
				Keys: nil,
				Name: "guardian_changed",
			},
			{
				Data: []*Event_Data{
					{
						Name: "new_guardian",
						Type: "felt",
					},
				},
				Keys: nil,
				Name: "guardian_backup_changed",
			},
			{
				Data: []*Event_Data{
					{
						Name: "active_at",
						Type: "felt",
					},
				},
				Keys: nil,
				Name: "escape_guardian_triggered",
			},
			{
				Data: []*Event_Data{
					{
						Name: "active_at",
						Type: "felt",
					},
				},
				Keys: nil,
				Name: "escape_signer_triggered",
			},
			{
				Data: nil,
				Keys: nil,
				Name: "escape_canceled",
			},
		},
		Structs: []*Struct{
			{
				Fields: []*Struct_Field{
					{
						Name:   "to",
						Type:   "felt",
						Offset: 0,
					},
					{
						Name:   "selector",
						Type:   "felt",
						Offset: 1,
					},
					{
						Name:   "data_offset",
						Type:   "felt",
						Offset: 2,
					},
					{
						Name:   "data_len",
						Type:   "felt",
						Offset: 3,
					},
				},
				Name: "CallArray",
				Size: 4,
			},
		},
		L1Handlers: []*Function{
			{
				Name: "__l1_default__",
				Inputs: []*Function_Input{
					{
						Name: "selector",
						Type: "felt",
					},
					{
						Name: "calldata_size",
						Type: "felt",
					},
					{
						Name: "calldata",
						Type: "felt*",
					},
				},
				Outputs: nil,
			},
		},
		Constructor: &Function{
			Name: "constructor",
			Inputs: []*Function_Input{
				{
					Name: "signer",
					Type: "felt",
				},
				{
					Name: "guardian",
					Type: "felt",
				},
			},
			Outputs: nil,
		},
	},
}
