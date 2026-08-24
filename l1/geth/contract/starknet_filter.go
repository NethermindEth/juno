package contract

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
)

func (_Starknet *StarknetFilterer) FilterLogStateUpdate(
	opts *bind.FilterOpts,
) ([]*StarknetLogStateUpdate, error) {
	logs, sub, err := _Starknet.contract.FilterLogs(opts, "LogStateUpdate")
	if err != nil {
		return nil, err
	}
	defer sub.Unsubscribe()

	var out []*StarknetLogStateUpdate
	unpack := func(log types.Log) error {
		ev := new(StarknetLogStateUpdate)
		if err := _Starknet.contract.UnpackLog(ev, "LogStateUpdate", log); err != nil {
			return err
		}
		ev.Raw = log
		out = append(out, ev)
		return nil
	}
	for {
		select {
		case log := <-logs:
			if err := unpack(log); err != nil {
				return nil, err
			}
		case err := <-sub.Err():
			// bind.BoundContract.FilterLogs ships logs from a goroutine that
			// closes sub.Err() when done but never closes the logs channel.
			// On graceful close (err == nil) drain remaining buffered logs.
			if err != nil {
				return nil, err
			}
			for {
				select {
				case log := <-logs:
					if err := unpack(log); err != nil {
						return nil, err
					}
				default:
					return out, nil
				}
			}
		}
	}
}
