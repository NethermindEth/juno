#!/usr/bin/env python3
from starkware.cairo.lang.vm.crypto import pedersen_hash_func
from starkware.starknet.business_logic.state.state import SharedState, StateSelector
from starkware.starknet.definitions.general_config import StarknetGeneralConfig
from starkware.starknet.testing.state import StarknetState
from starkware.starkware_utils.commitment_tree.patricia_tree import PatriciaTree
from starkware.storage.dict_storage import DictStorage
from starkware.storage.storage import FactFetchingContext


async def call():
  storage = {}
  config = StarknetGeneralConfig()
  context = FactFetchingContext(DictStorage(storage), pedersen_hash_func)

  root = b""
  info = ""
  addr = ""

  shared = SharedState(PatriciaTree(root, 251), info)
  carried = await shared.get_filled_carried_state(context, StateSelector(addr))
  state = StarknetState(carried, config)

  selector = ""
  caller = ""
  sig = ""
  result = await state.invoke_raw(addr, selector, caller, 0, sig)

  return result.call_info.retdata

if __name__ == "__main__":
  call()
