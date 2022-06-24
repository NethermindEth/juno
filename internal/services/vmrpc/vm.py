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


def hex_str_to_int(hex_str):
    """Returns an int from a string hexadecimal representation of a
    number. The function is agnostic to whether the string has a 0x
    prefix.
    """
    return int(hex_str, 16)


def hex_str_list_to_int_list(hex_str_list):
    """Returns an [int] where the contents are strings that form a
    hexadecimal representation of a number. The function is agnostic to
    whether the string has a 0x prefix.
    """
    return list(map(hex_str_to_int, hex_str_list))


def int_to_bytes(n):
    """Takes an int and returns a 32-byte string representation of said
    int."""
    return n.to_bytes(32, "big")


def int_list_to_hex_str_list(int_list):
    """Takes a list of ints and returns a list of 32-byte string
    representations of the contents."""
    return list(map(hex, int_list))


def split_key(key):
    """Takes a byte string of the form prefix:suffix where the suffix is
    encoded as printable ASCII characters and returns a tuple of
    (prefix, suffix) where additionally the suffix has been converted
    into a byte representation of the hex characters which can be easily
    read by Go in order to execute database queries.
    """
    prefix, suffix = key.split(b":", maxsplit=1)
    return prefix, bytes(suffix.hex(), "ascii")


async def call(
    adapter,
    calldata,
    caller_address,
    contract_address,
    root,
    selector,
):
    # Parse values.
    calldata = hex_str_list_to_int_list(calldata)
    caller_address = hex_str_to_int(caller_address)
    contract_address = hex_str_to_int(contract_address)
    root = int_to_bytes(hex_str_to_int(root))
    selector = hex_str_to_int(selector)

    # XXX: Sequencer's address. This does not appear to be important so
    # a dummy value could suffice.
    sequencer = 0x37B2CD6BAAA515F520383BEE7B7094F892F4C770695FC329A8973E841A971AE

    shared_state = SharedState(
        contract_states=PatriciaTree(root=root, height=251),
        block_info=BlockInfo.empty(sequencer_address=sequencer),
    )
    carried_state = await shared_state.get_filled_carried_state(
        ffc=FactFetchingContext(storage=adapter, hash_func=crypto.pedersen_hash_func),
        state_selector=StateSelector(contract_addresses={contract_address}),
    )

    state = StarknetState(state=carried_state, general_config=StarknetGeneralConfig())
    result = await state.call_raw(
        contract_address=contract_address,
        selector=selector,
        calldata=calldata,
        caller_address=caller_address,
        max_fee=0,
    )
    return int_list_to_hex_str_list(result.retdata)


class StorageRPCClient(Storage):
    def __init__(self, juno_address) -> None:
        self.juno_address = juno_address

    async def set_value(self, key, value):
        raise NotImplementedError("Only read operations are supported.")

    async def del_value(self, key):
        raise NotImplementedError("Only read operations are supported.")

    async def get_value(self, key):
        prefix, suffix = split_key(key)
        # Leaf keys encode their value in the suffix so there is no need
        # to make the database call.
        if prefix == b"starknet_storage_leaf":
            return bytes.fromhex(suffix.decode("ascii"))
        async with grpc.aio.insecure_channel(self.juno_address) as channel:
            stub = vm_pb2_grpc.StorageAdapterStub(channel)
            request = vm_pb2.GetValueRequest(key=prefix + b":" + suffix)
            response = await stub.GetValue(request)
            # XXX: Using structural pattern matching here would probably
            # be a cleaner way to express the following but Python 3.10
            # does not play nice with cairo-lang.
            if prefix == b"contract_state" or prefix == b"contract_definition_fact":
                return response.value
            elif prefix == b"patricia_node":
                return bytes.fromhex(response.value.decode("ascii"))
            raise ValueError("invalid prefix: " + prefix.decode("ascii"))


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
