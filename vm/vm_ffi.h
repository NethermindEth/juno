#ifndef VM_FFI_H
#define VM_FFI_H

#include <stdint.h>
#include <stdlib.h>
#include <stddef.h>

#define FELT_SIZE 32

typedef struct CallInfo {
	unsigned char contract_address[FELT_SIZE];
	unsigned char class_hash[FELT_SIZE];
	unsigned char entry_point_selector[FELT_SIZE];
	unsigned char** calldata;
	size_t len_calldata;
} CallInfo;

typedef struct ChainInfo {
	char* chain_id;
	unsigned char eth_fee_token_address[FELT_SIZE];
	unsigned char strk_fee_token_address[FELT_SIZE];
} ChainInfo;

typedef struct BlockInfo {
	unsigned long long block_number;
	unsigned long long block_timestamp;
	unsigned char is_pending;
	unsigned char sequencer_address[FELT_SIZE];
	unsigned char l1_gas_price_wei[FELT_SIZE];
	unsigned char l1_gas_price_fri[FELT_SIZE];
	char* version;
	unsigned char block_hash_to_be_revealed[FELT_SIZE];
	unsigned char l1_data_gas_price_wei[FELT_SIZE];
	unsigned char l1_data_gas_price_fri[FELT_SIZE];
	unsigned char use_blob_data;
	unsigned char l2_gas_price_wei[FELT_SIZE];
	unsigned char l2_gas_price_fri[FELT_SIZE];
} BlockInfo;

extern void cairoVMCall(
	CallInfo* call_info_ptr, 
	BlockInfo* block_info_ptr, 
	ChainInfo* chain_info_ptr, 
	uintptr_t readerHandle,
	unsigned long long max_steps, 
	unsigned long long initial_gas,
	unsigned char concurrency_mode, 
	unsigned char err_stack, 
	unsigned char return_state_diff
);

extern void cairoVMExecute(
	char* txns_json, 
	char* classes_json, 
	char* paid_fees_on_l1_json,
	BlockInfo* block_info_ptr, 
	ChainInfo* chain_info_ptr, 
	uintptr_t readerHandle,
	unsigned char skip_charge_fee, 
	unsigned char skip_validate, 
	unsigned char err_on_revert,
	unsigned char concurrency_mode, 
	unsigned char err_stack,
	unsigned char allow_binary_search,
	unsigned char is_estimate_fee,
	unsigned char return_initial_reads
);

extern char* setVersionedConstants(char* json);
extern void freeString(char* str);

#endif // VM_FFI_H

