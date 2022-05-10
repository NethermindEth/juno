package db

// KeyValueStore implement the Storer interface that use a Databaser
type KeyValueStore struct {
	db         *Databaser
	prefix     []byte
	operations Operations
}

func NewKeyValueStore(db *Databaser, prefix string) KeyValueStore {
	return KeyValueStore{db: db, prefix: []byte(prefix), operations: NewOperations()}
}

func (k KeyValueStore) Delete(key []byte) {
	err := (*k.db).Delete(append(k.prefix, key...))
	if err != nil {
		return
	}
}

func (k KeyValueStore) Get(key []byte) ([]byte, bool) {
	//log.Default.With("Key", string(append(k.prefix, key...))).Info("Getting keys from db")
	//val, ok := k.ephemeral.Get(key)
	get, err := (*k.db).Get(append(k.prefix, key...))
	if err != nil {
		return nil, false
	}
	//if string(val) != string(get) || (ok != (get != nil)) {
	//	log.Default.With("Ephemeral", val, "Lmdbx", get, "Key", string(key)).Panic("values aren't equals")
	//}
	return get, get != nil
}

func (k KeyValueStore) Put(key, val []byte) {
	//log.Default.With("Value", string(val), "Key", string(append(k.prefix, key...))).
	//	Info("Putting values on db")
	//k.ephemeral.Put(key, val)
	err := (*k.db).Put(append(k.prefix, key...), val)
	if err != nil {
		panic(any("Error"))
		return
	}
}

func (k KeyValueStore) Init() {

}

func (k KeyValueStore) Persist() {

}
