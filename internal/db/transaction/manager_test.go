package transaction

import (
	"github.com/NethermindEth/juno/internal/db"
	"testing"
)

var txs = []*Transaction{
	{
		Hash: "49eb3544a95587518b0d2a32b9e456cb05b32e0085ebc0bcecb8ef2e15dc3a2",
		Tx: &Transaction_Invoke{Invoke: &InvokeFunction{
			ContractAddress:    "7e1b2de3dc9e3cf83278452786c23b384cf77a66c3073f94ab451ed0029b5af",
			EntryPointSelector: "317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f",
			CallData: []string{
				"1d24663eb96a2b9a568b88ff520d04779c03ae8ce3087aa55bea1a34f07c6f7",
				"2",
				"1ab006325ae5196978cfcaefd3d748e8583079ebb33b402537a4b4c174e16c6",
				"77575e21a0326adb26b58aaa1ef7139ba8e3164ab54411dbf9a5809b8d6ea8",
			},
			Signature: nil,
			MaxFee:    "0",
		}},
	},
	{
		Hash: "50398c6ec05a07642e5bd52c656e1650f3b057361283ecbb19d4062199e4626",
		Tx: &Transaction_Invoke{Invoke: &InvokeFunction{
			ContractAddress:    "3e875a858f9a0229e4a59cb72a4086d324b9b2148242694f2dd12d59d993b62",
			EntryPointSelector: "27c3334165536f239cfd400ed956eabff55fc60de4fb56728b6a4f6b87db01c",
			CallData: []string{
				"4d6f00affbeb6239fe0eb3eb4afefddbaea71533c152f44a1cdd113c1fdeade",
				"33ce93a3eececa5c9fc70da05f4aff3b00e1820b79587924d514bc76788991a",
				"1",
				"0",
			},
			Signature: nil,
			MaxFee:    "0",
		}},
	},
	{
		Hash: "1209ae3031dd69ef8ab4507dc4cc2c478d9a0414cb42225ce223670dee5cdcf",
		Tx: &Transaction_Invoke{Invoke: &InvokeFunction{
			ContractAddress:    "764c36cfdc456e1f3565441938f958badcc0ce8f20b7ed5819af30ed18f245",
			EntryPointSelector: "317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f",
			CallData: []string{
				"7e24f78ee360727e9fdd55e2b847202057724bf1f7e5bc25ff78f7760b6895b",
				"2",
				"51378ba07a08230eab5af933c8e1bd905bc9436bf96ab5f173010eb022eb2a4",
				"5f8a361ec261cb4b34d4481803903bb9b8e5c8768e24099aa85ad7f3e8f13b8",
			},
			Signature: nil,
			MaxFee:    "0",
		}},
	},
	{
		Hash: "e0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75",
		Tx: &Transaction_Deploy{Deploy: &Deploy{
			ContractAddressSalt: "546c86dc6e40a5e5492b782d8964e9a4274ff6ecb16d31eb09cee45a3564015",
			ConstructorCallData: []string{
				"06cf6c2f36d36b08e591e4489e92ca882bb67b9c39a3afccf011972a8de467f0",
				"7ab344d88124307c07b56f6c59c12f4543e9c96398727854a322dea82c73240",
			},
		}},
	},
	{
		Hash: "12c96ae3c050771689eb261c9bf78fac2580708c7f1f3d69a9647d8be59f1e1",
		Tx: &Transaction_Deploy{Deploy: &Deploy{
			ContractAddressSalt: "12afa0f342ece0468ca9810f0ea59f9c7204af32d1b8b0d318c4f2fe1f384e",
			ConstructorCallData: []string{
				"cfc2e2866fd08bfb4ac73b70e0c136e326ae18fc797a2c090c8811c695577e",
				"5f1dd5a5aef88e0498eeca4e7b2ea0fa7110608c11531278742f0b5499af4b3",
			},
		}},
	},
}

func TestManager_PutTransaction(t *testing.T) {
	database := db.NewKeyValueDb(t.TempDir(), 0)
	manager := NewManager(database)
	for _, tx := range txs {
		manager.PutTransaction(tx.Hash, tx)
	}
	manager.Close()
}

func TestManager_GetTransaction(t *testing.T) {
	database := db.NewKeyValueDb(t.TempDir(), 0)
	manager := NewManager(database)
	// Insert all the transactions
	for _, tx := range txs {
		manager.PutTransaction(tx.Hash, tx)
	}
	// Get all the transactions and compare
	for _, tx := range txs {
		outTx := manager.GetTransaction(tx.Hash)
		if !tx.Equal(outTx) {
			t.Errorf("transaction not equal after Put/Get operations")
		}
	}
	manager.Close()
}
