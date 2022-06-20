import asyncio
import sys
import traceback

import grpc
from starkware.cairo.lang.vm import crypto
from starkware.starknet.business_logic.state.state import (BlockInfo,
                                                           SharedState,
                                                           StateSelector)
from starkware.starknet.definitions.general_config import StarknetGeneralConfig
from starkware.starknet.testing.state import StarknetState
from starkware.starkware_utils.commitment_tree.patricia_tree.patricia_tree import \
    PatriciaTree
from starkware.storage.storage import FactFetchingContext, Storage

import vm_pb2
import vm_pb2_grpc


# Helpers.
def parse_int(hex_str):
    return int(hex_str, 16)

def parse_int_list(hex_str_list):
    tmp = []
    for hex_str in hex_str_list:
        tmp.append(parse_int(hex_str=hex_str))
    return tmp

async def call(
    adapter,
    calldata,
    caller_address,
    contract_address,
    root,
    selector,
):
    calldata = parse_int_list(calldata)
    caller_address = parse_int(caller_address)
    contract_address = parse_int(contract_address)
    selector = parse_int(selector)

    # XXX: Sequencer's address. This does not appear to be important so
    # a dummy value could suffice.
    sequencer = 0x37b2cd6baaa515f520383bee7b7094f892f4c770695fc329a8973e841a971ae

    # context = FactFetchingContext(
    #     storage=adapter, hash_func=crypto.pedersen_hash_func)
    # shared = SharedState(
    #     contract_states=PatriciaTree(root=bytes.fromhex(root.removeprefix("0x")), height=251), 
    #     block_info=BlockInfo.empty(sequencer_address=sequencer))
    # carried = await shared.get_filled_carried_state(
    #     ffc=context, state_selector=StateSelector(contract_addresses={contract_address}))
    # state = StarknetState(state=carried, general_config=StarknetGeneralConfig())
    # result = await state.call_raw(
    #     contract_address=contract_address,
    #     selector=selector,
    #     calldata=calldata,
    #     caller_address=caller_address,
    #     max_fee=0,
    # )

    # return result.retdata
    return [await adapter.get_value(b"hello"), await adapter.get_value(b"world")]


class StorageRPCClient(Storage):
    def __init__(self, juno_address) -> None:
        self.juno_address = juno_address

    async def set_value(self, key, value):
        raise NotImplementedError("Only read operations are supported.")

    async def del_value(self, key):
        raise NotImplementedError("Only read operations are supported.")

    async def get_value(self, key):
        async with grpc.aio.insecure_channel(self.juno_address) as channel:
            stub = vm_pb2_grpc.StorageAdapterStub(channel)
            request = vm_pb2.GetValueRequest(key=key)
            response = await stub.GetValue(request)
            return response.value


class VMServicer(vm_pb2_grpc.VMServicer):
    def __init__(self, storage):
        self.storage = storage

    async def Call(self, request, context):
        try:
            return vm_pb2.VMCallResponse(
                retdata=await call(
                    adapter=self.storage,
                    calldata=request.calldata,
                    caller_address=request.caller_address,
                    contract_address=request.contract_address,
                    root=request.root,
                    selector=request.selector,
                )
            )
        except Exception as e:
            traceback.print_exception(type(e), e, e.__traceback__)
            raise e


async def serve(listen_address: str, juno_address: str) -> None:
    server = grpc.aio.server()
    vm_pb2_grpc.add_VMServicer_to_server(
        VMServicer(StorageRPCClient(juno_address)), server
    )
    server.add_insecure_port(listen_address)
    await server.start()
    await server.wait_for_termination()


if __name__ == "__main__":
    asyncio.run(serve(sys.argv[1], sys.argv[2]))
