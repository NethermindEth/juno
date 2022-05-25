# Contract deployment details on Goerli.
#
# Block hash      : 0x1dcb1ec71970798db8ad14743868258a536ad662ec07bc0cc23a495389a48e3
# Contract address: 0x04bedcd144c98a73fcee66dfe7ec3669086b6e8f89ef33bb2f397993e1bb90be
# Transaction hash: 0x2e7624858e840d73a88fa15c21b9eb07af3b9826be1e5c673b30a60a8d247c3
#
#
# Contract invocation details on Goerli.
#
# Block hash      : 0x4f9cc0e6168cf3f6c7eaf7b93f2e550473faf4e40c076930893d032896ab331
# Contract address: 0x04bedcd144c98a73fcee66dfe7ec3669086b6e8f89ef33bb2f397993e1bb90be
# Transaction hash: 0x2d4b1a8af29d9d4902aeac1877cab36dd16245d59ff24304c214e80d98681ed

# Declare this file as a StarkNet contract.
%lang starknet

from starkware.cairo.common.cairo_builtins import HashBuiltin

# Define a storage variable.
@storage_var
func balance() -> (res : felt):
end

# Increases the balance by the given amount.
@external
func increase_balance{
  syscall_ptr : felt*, pedersen_ptr : HashBuiltin*, range_check_ptr,
}(
  amount : felt,
):
  let (res) = balance.read()
  balance.write(res + amount)
  return ()
end

# Returns the current balance.
@view
func get_balance{
  syscall_ptr : felt*, pedersen_ptr : HashBuiltin*, range_check_ptr,
}() -> (
  res : felt,
):
  let (res) = balance.read()
  return (res)
end
