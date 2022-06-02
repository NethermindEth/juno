%lang starknet

from starkware.starknet.common.syscalls import (storage_read, storage_write)

@contract_interface
namespace MyContract:
    func increase_value(address : felt, value : felt):
    end
end

@external
func increase_value{syscall_ptr : felt*}(address : felt, value : felt):
    let (res) = storage_read(address=address)
    return storage_write(address=address, value=res + value)
end

@external
func call_increase_value{syscall_ptr : felt*, range_check_ptr}(
        contract_address : felt, address : felt, value : felt):
    MyContract.increase_value(contract_address=contract_address, address=address, value=value)
    return ()
end

@external
func get_value{syscall_ptr : felt*}(address : felt) -> (res : felt):
    return storage_read(address=address)
end
