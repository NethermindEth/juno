import asyncio
import sys
import traceback

import grpc
from starkware.cairo.lang.vm import crypto
from starkware.starknet.business_logic.state.state import (
    BlockInfo,
    SharedState,
    StateSelector,
)
from starkware.starknet.definitions.general_config import StarknetGeneralConfig
from starkware.starknet.testing.state import StarknetState
from starkware.starkware_utils.commitment_tree.patricia_tree.patricia_tree import (
    PatriciaTree,
)
from starkware.storage.storage import FactFetchingContext, Storage

import vm_pb2
import vm_pb2_grpc


# Helpers.

def parse_int(hex_str):
    """Returns an int from a string hexadecimal representation of a
    number. The function is agnostic to whether the string has a 0x
    prefix or not.
    """
    return int(hex_str, 16)


def parse_int_list(hex_str_list):
    """Returns an [int] where the contents are strings that form a
    hexadecimal representation of a number. The function is agnostic to
    whether the string has a 0x prefix or not.
    """
    tmp = []
    for hex_str in hex_str_list:
        tmp.append(parse_int(hex_str=hex_str))
    return tmp


def serialise_key(key):
    """Takes a byte string of the form prefix:suffix where the suffix is
    encoded as printable ASCII characters and returns prefix:suffix
    where the suffix has been converted into a byte representation of
    the hex characters which can be easily read by Go in order to
    execute database queries.
    """
    separator = b":"
    prefix, suffix = key.split(separator, maxsplit=1)
    return prefix + separator +  bytes(suffix.hex(), "ascii")


def deserialise_val(value):
    raise NotImplementedError


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
    return [
        await adapter.get_value(b"patricia_node:\x07\x04\xdf\xcb\xc4p7|h\xe6\xf5\xff\xb89p\xeb\xd0\xd7\xc4\x8d[\x8d/N\xd6\x1a$\xe7\x95\xe04\xbd"),
        await adapter.get_value(b"contract_state:\x00.\x97#\xe5G\x11\xae\xc5n?\xb6\xad\x1b\xb8'/d\xec\x92\xe0\xa4: \xfe\xed\x94;\x1dOs\xc5")
    ]


class StorageRPCClient(Storage):
    def __init__(self, juno_address) -> None:
        self.juno_address = juno_address

    async def set_value(self, key, value):
        raise NotImplementedError("Only read operations are supported.")

    async def del_value(self, key):
        raise NotImplementedError("Only read operations are supported.")

    async def get_value(self, key):
        serialised_key = serialise_key(key)
        async with grpc.aio.insecure_channel(self.juno_address) as channel:
            stub = vm_pb2_grpc.StorageAdapterStub(channel)
            request = vm_pb2.GetValueRequest(key=serialised_key)
            response = await stub.GetValue(request)
            # TODO: Conversely, the value has to be converted into bytes
            # before is is read by the virtual machine using
            # bytes.fromhex in deserialise_val above.
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
