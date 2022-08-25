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
        adapter=None,
        calldata=None,
        caller_address=None,
        class_hash=None,
        contract_address=None,
        root=None,
        selector=None,
        sequencer=None,
):
    shared_state = SharedState(
        contract_states=PatriciaTree(root=root, height=251),
        block_info=BlockInfo.empty(sequencer_address=sequencer),
    )
    carried_state = await shared_state.get_filled_carried_state(
        ffc=FactFetchingContext(storage=adapter, hash_func=crypto.pedersen_hash_func),
        state_selector=StateSelector(
            contract_addresses={contract_address}, class_hashes={class_hash}
        ),
    )
    state = StarknetState(state=carried_state, general_config=StarknetGeneralConfig())
    result = await state.call_raw(
        contract_address=contract_address,
        selector=selector,
        calldata=calldata,
        caller_address=caller_address,
        max_fee=0,
    )
    return result[0].retdata


class StorageRPCClient(Storage):
    def __init__(self, juno_address) -> None:
        self.juno_address = juno_address

    async def set_value(self, key, value):
        raise NotImplementedError("Only read operations are supported.")

    async def del_value(self, key):
        raise NotImplementedError("Only read operations are supported.")

    async def get_value(self, key):
        prefix, suffix = split_key(key)
        async with grpc.aio.insecure_channel(self.juno_address) as channel:
            stub = vm_pb2_grpc.StorageAdapterStub(channel)
            suffix = bytes.fromhex(suffix.decode("ascii"))
            request = vm_pb2.GetValueRequest(key=suffix)
            if prefix == b'patricia_node':
                response = await stub.GetPatriciaNode(request)
                if response.type == vm_pb2.NodeType.EDGE_NODE:
                    return (
                            response.bottom.rjust(32, b'\00')
                            + response.path.rjust(32, b'\00')
                            + response.len.to_bytes(1, "big")
                    )
                elif response.type == vm_pb2.NodeType.BINARY_NODE:
                    return response.left.rjust(32, b'\00') + response.right.rjust(32, b'\00')
            elif prefix == b"contract_state":
                response = await stub.GetContractState(request)
                return (
                        b'{"storage_commitment_tree": {"root": "'
                        + response.storageRoot.hex().encode("utf-8")
                        + b'", "height": 251}, "contract_hash": "'
                        + response.contractHash.hex().encode("utf-8")
                        + b'"}'
                )
            elif prefix == b"contract_definition_fact":
                response = await stub.GetContractDefinition(request)
                return b'{"contract_definition":' + response.value + b"}"
            elif prefix == b"starknet_storage_leaf":
                return suffix
            else:
                raise ValueError(f"Unknown prefix: {prefix}")


class VMServicer(vm_pb2_grpc.VMServicer):
    def __init__(self, storage):
        self.storage = storage

    async def Call(self, request, context):
        try:
            # Parse values.
            calldata = [int.from_bytes(c, byteorder="big") for c in request.calldata]
            caller_address = int.from_bytes(request.caller_address, byteorder="big")
            contract_address = int.from_bytes(request.contract_address, byteorder="big")
            class_hash = request.class_hash
            root = request.root
            selector = int.from_bytes(request.selector, byteorder="big")
            sequencer = int.from_bytes(request.sequencer, byteorder="big")

            retdata = await call(
                adapter=self.storage,
                calldata=calldata,
                caller_address=caller_address,
                contract_address=contract_address,
                class_hash=class_hash,
                root=root,
                selector=selector,
                sequencer=sequencer,
            )
            return vm_pb2.VMCallResponse(
                retdata=[c.to_bytes(32, byteorder="big") for c in retdata]
            )
        except Exception as e:
            traceback.print_exception(type(e), e, e.__traceback__)
            raise e from None


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
