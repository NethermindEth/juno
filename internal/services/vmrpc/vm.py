import asyncio
import sys
import traceback

import grpc
from starkware.cairo.lang.vm import crypto
from starkware.starknet.business_logic.state.state import BlockInfo, SharedState, StateSelector
from starkware.starknet.definitions.general_config import StarknetGeneralConfig
from starkware.starknet.testing.state import StarknetState
from starkware.starkware_utils.commitment_tree.patricia_tree.patricia_tree import PatriciaTree
from starkware.storage.storage import FactFetchingContext, Storage

import vm_pb2
import vm_pb2_grpc


async def call(
    # XXX: storage is not needed if calls to storage are going to be 
    # handled directly by the adapter.
    # storage,
    # XXX: BlockInfo does not appear to be important here.
    # block_info=None,
    calldata,
    caller_address,
    contract_address,
    # XXX: The compiled contract. See services.getFullContract in the 
    # vm_utils.go file.
    contract_definition,
    root,
    selector,
    # XXX: signature does not appear to be important as well.
    # signature,
):
    # TODO: How to get the contract's hash?

    # TODO: What is juno_addr? (See StorageRPCClient constructor).
    juno_addr = ""
    adapter = StorageRPCClient(juno_addr)

    # XXX: Sequencer's address. This does not appear to be important so
    # a dummy value could suffice. 
    sequencer = 0x37b2cd6baaa515f520383bee7b7094f892f4c770695fc329a8973e841a971ae

    context = FactFetchingContext(storage=adapter, hash_func=crypto.pedersen_hash_func)
    shared = SharedState(contract_states=PatriciaTree(root=root, height=251), block_info=BlockInfo.empty(sequencer_address=sequencer))
    carried = await shared.get_filled_carried_state(ffc=context, state_selector=StateSelector(contract_addresses={contract_address}))
    state = StarknetState(state=carried, general_config=StarknetGeneralConfig())
    result = await state.call_raw(
        contract_address=contract_address,
        selector=selector,
        calldata=calldata,
        caller_address=caller_address,
        max_fee=0,
    )

    return result.retdata


class StorageRPCClient(Storage):
    def __init__(self, juno_address) -> None:
        self.juno_address = juno_address

    async def set_value(self, key, value):
        # XXX: Only read-only operations are supported right now.
        raise NotImplementedError

    async def del_value(self, key):
        # XXX: Only read-only operations are supported right now.
        raise NotImplementedError

    async def get_value(self, key):
        # TODO: Because the contract definition is not stored locally,
        # a request whose key starts with b"contract_definition_fact"
        # should be intercepted. For example, this class can be 
        # instantiated with a Dict that where 
        # key = b"contract_definition_fact:\x00 ..." and \x00 is the
        # contract's hash and val = compiled contract code which is also
        # passed into the call function above as contract_definition.
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
