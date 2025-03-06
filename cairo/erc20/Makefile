build: ## `scarb build` generates json with `abi` field as array, we need to convert it to string
	scarb build
	jq '.abi = ( .abi | tostring )' target/dev/erc20_ERC20Upgradeable.contract_class.json > tmp.json 
	mv tmp.json target/dev/erc20_ERC20Upgradeable.contract_class.json
	
clean: ## Clean the project
	scarb clean
