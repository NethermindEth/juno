package class

import (
	_ "embed"
	"encoding/json"
	"math/big"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"

	"gotest.tools/assert"

	"github.com/NethermindEth/juno/pkg/felt"

	"github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/pkg/feeder"

	"github.com/NethermindEth/juno/internal/db"
)

var (
	//go:embed internal/class_def.json
	classDef  string
	classHash = new(felt.Felt).SetHex("0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8")
)

func TestClassManager(t *testing.T) {
	cm := initTestClassManager(t)
	defer cm.Close()

	var fullContract feeder.FullContract
	if err := json.Unmarshal([]byte(classDef), &fullContract); err != nil {
		t.Fatal(err)
	}

	c, err := types.NewContractClassFromFeeder(&fullContract)
	if err != nil {
		t.Fatal(err)
	}

	if err := cm.PutClass(classHash, c); err != nil {
		t.Fatal(err)
	}

	got, err := cm.GetClass(classHash)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, c, got, gocmp.Comparer(func(x, y *big.Int) bool {
		return x.Cmp(y) == 0
	}))
}

func initTestClassManager(t *testing.T) ClassManager {
	t.Helper()
	env, err := db.NewMDBXEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Fail()
	}
	classDb, err := db.NewMDBXDatabase(env, "CLASS")
	if err != nil {
		t.Fail()
	}
	return NewClassManager(classDb)
}
