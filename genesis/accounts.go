package genesis

import (
	"github.com/NethermindEth/juno/core/felt"
)

type account struct {
	PubKey felt.Felt
	PrivKey felt.Felt
	Address felt.Felt
}

func Accounts()([]account){
		
	strToFelt := func(feltStr string) felt.Felt{
		felt,_:= new(felt.Felt).SetString(feltStr)		
		return *felt
	}

	// deploy account
	return []account{
		{
			PubKey: strToFelt("0x16d03d341717ab11083a481f53278d8e54f610af815cbdab4035b2df283fcc0"),
			PrivKey: strToFelt("0x2bff1b26236b72d8a930be1dfbee09f79a536a49482a4c8b8f1030e2ab3bf1b"),
			Address: strToFelt("0x101"),
		},
		{
			PubKey: strToFelt("0x3bc7ab4ca475e24a0053db47c3e5a2a53264a30639d8b2bd0a08407da3ca0c"),
			PrivKey: strToFelt("0x43d8de30e55ed83b4436aea47e7517d4a52d06912938e2887cb1d33518daef1"),
			Address: strToFelt("0x102"),
		},
	}
}