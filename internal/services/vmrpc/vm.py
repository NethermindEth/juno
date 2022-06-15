import sys
import traceback
import asyncio
import grpc
import vm_pb2
import vm_pb2_grpc
from starkware.storage.storage import Storage
from starkware.starknet.business_logic.state.state import (
    BlockInfo,
    SharedState,
    StateSelector,
)
from starkware.starknet.definitions.general_config import StarknetGeneralConfig
from starkware.storage.storage import FactFetchingContext
from starkware.starkware_utils.commitment_tree.patricia_tree.patricia_tree import (
    PatriciaTree,
)
from starkware.cairo.lang.vm.crypto import pedersen_hash_func
from starkware.starknet.testing.state import StarknetState


async def call(
    storage,
    root,
    contract_address,
    selector,
    calldata,
    caller_address,
    signature,
    block_info=None,
):

    # general_config = StarknetGeneralConfig()
    #
    # # hook up the juno storage over rpc
    # ffc = FactFetchingContext(storage=storage, hash_func=pedersen_hash_func)
    #
    # # the root tree has to always be height=251
    # shared_state = SharedState(PatriciaTree(root=root, height=251), block_info)
    # state_selector = StateSelector(
    #     contract_addresses={contract_address}, class_hashes=set()
    # )
    # carried_state = await shared_state.get_filled_carried_state(
    #     ffc, state_selector=state_selector
    # )
    #
    # state = StarknetState(state=carried_state, general_config=general_config)
    # max_fee = 0
    #
    # output = await state.invoke_raw(
    #     contract_address, selector, calldata, caller_address, max_fee, signature
    # )
    #
    # # this is everything we need, at least so far for the "call".
    # return output.call_info.retdata

    return [await storage.get_value(b"hello"), await storage.get_value(b"world")]


class StorageRPCClient(Storage):
    def __init__(self, juno_address) -> None:
        self.juno_address = juno_address

    async def set_value(self, key, value):
        # call juno.set_value
        pass

    async def del_value(self, key):
        # call juno.del_value
        pass

    async def get_value(self, key):
        # call juno.get_value
        async with grpc.aio.insecure_channel(self.juno_address) as channel:
            stub = vm_pb2_grpc.StorageAdapterStub(channel)
            reqest = vm_pb2.GetValueRequest(key=key)
            response = await stub.GetValue(reqest)
            return response.value


class VMServicer(vm_pb2_grpc.VMServicer):
    def __init__(self, storage):
        self.storage = storage

    async def Call(self, request, context):
        try:
            return vm_pb2.VMCallResponse(
                retdata=await call(
                    self.storage,
                    request.root,
                    request.contract_address,
                    request.selector,
                    request.calldata,
                    request.caller_address,
                    request.signature,
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
