import asyncio
import os

from starkware.cairo.lang.vm import crypto
from starkware.starknet.business_logic.state.objects import ContractDefinitionFact
from starkware.starknet.business_logic.state.state import BlockInfo, SharedState, StateSelector
from starkware.starknet.compiler import compile
from starkware.starknet.definitions.general_config import StarknetGeneralConfig
from starkware.starknet.testing.state import StarknetState
from starkware.starkware_utils.commitment_tree.patricia_tree.patricia_tree import PatriciaTree
from starkware.storage.dict_storage import DictStorage
from starkware.storage.storage import FactFetchingContext


async def call():
  # TODO: Need to populate this storage directly from the database.
  # Injected storage.
  storage = {
    # XXX: Root node of the state trie i.e. after doing the insertion
    # key (contract address) = 57dde83c18c0efe7123c36a52d704cf27d5c38cdf0b1e1edc3b0dae3ee4e374
    # and value = h(h(h(contract_hash, storage_root), 0), 0) (see 
    # below).
    # key = patricia_node:0704dfcbc470377c68e6f5ffb83970ebd0d7c48d5b8d2f4ed61a24e795e034bd
    # val = 002e9723e54711aec56e3fb6ad1bb8272f64ec92e0a43a20feed943b1d4f73c5057dde83c18c0efe7123c36a52d704cf27d5c38cdf0b1e1edc3b0dae3ee4e374fb
    b'patricia_node:\x07\x04\xdf\xcb\xc4p7|h\xe6\xf5\xff\xb89p\xeb\xd0\xd7\xc4\x8d[\x8d/N\xd6\x1a$\xe7\x95\xe04\xbd': b"\x00.\x97#\xe5G\x11\xae\xc5n?\xb6\xad\x1b\xb8'/d\xec\x92\xe0\xa4: \xfe\xed\x94;\x1dOs\xc5\x05}\xde\x83\xc1\x8c\x0e\xfeq#\xc3jR\xd7\x04\xcf'\xd5\xc3\x8c\xdf\x0b\x1e\x1e\xdc;\r\xae>\xe4\xe3t\xfb",

    # XXX: The key is h(h(h(contract_hash, storage_root), 0), 0) which 
    # is the value associated with the address (the address being the 
    # key) in the state trie.
    # key = contract_state:002e9723e54711aec56e3fb6ad1bb8272f64ec92e0a43a20feed943b1d4f73c5
    b"contract_state:\x00.\x97#\xe5G\x11\xae\xc5n?\xb6\xad\x1b\xb8'/d\xec\x92\xe0\xa4: \xfe\xed\x94;\x1dOs\xc5": b'{"storage_commitment_tree": {"root": "04fb440e8ca9b74fc12a22ebffe0bc0658206337897226117b985434c239c028", "height": 251}, "contract_hash": "050b2148c0d782914e0b12a1a32abe5e398930b7e914f82c65cb7afce0a0ab9b"}',

    # XXX: Root node of the contract storage trie where the insertion
    # trie.Put(132, 3) has been made.
    # key = 04fb440e8ca9b74fc12a22ebffe0bc0658206337897226117b985434c239c028
    # val = 00000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000084fb 
    b'patricia_node:\x04\xfbD\x0e\x8c\xa9\xb7O\xc1*"\xeb\xff\xe0\xbc\x06X c7\x89r&\x11{\x98T4\xc29\xc0(': b'\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x03\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x84\xfb',

    # key = 0000000000000000000000000000000000000000000000000000000000000003
    b'starknet_storage_leaf:\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x03': b'\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x03'
  }

  # Compile the contract.
  source = os.path.join(os.path.dirname(__file__), "test.cairo")
  definition = compile.compile_starknet_files([source], True)
  raw = ContractDefinitionFact(definition).dumps().encode("ascii")

  # Contract's hash.
  fact_key = b"\x05\x0b!H\xc0\xd7\x82\x91N\x0b\x12\xa1\xa3*\xbe^9\x890\xb7\xe9\x14\xf8,e\xcbz\xfc\xe0\xa0\xab\x9b"
  # XXX: Update the storage with the contract definition.
  key = ContractDefinitionFact.db_key(fact_key)
  storage[key] = raw

  # State root.
  root = bytes.fromhex("0704dfcbc470377c68e6f5ffb83970ebd0d7c48d5b8d2f4ed61a24e795e034bd")

  # Contract's address.
  addr  = 0x57dde83c18c0efe7123c36a52d704cf27d5c38cdf0b1e1edc3b0dae3ee4e374

  # "get_value" in the Cairo contract takes an argument that specifies
  # an address to return the value from.
  calldata = [132]

  # Caller's address.
  caller_addr = 0

  # Sequencer's address.
  sequencer = 0x37b2cd6baaa515f520383bee7b7094f892f4c770695fc329a8973e841a971ae

  # Maximum fee.
  max_fee = 0

  context = FactFetchingContext(DictStorage(storage), crypto.pedersen_hash_func)
  shared = SharedState(PatriciaTree(root, 251), BlockInfo.empty(sequencer))
  carried = await shared.get_filled_carried_state(context, StateSelector({addr}))
  state = StarknetState(carried, StarknetGeneralConfig())
  result = await state.call_raw(
    addr,
    "get_value",
    calldata,
    caller_addr,
    max_fee,
  )

  return result.retdata


if __name__ == "__main__":
  res = asyncio.run(call())
  print(res)
