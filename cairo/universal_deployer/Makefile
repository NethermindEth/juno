build: ## `scarb build` generates json with `abi` field as array, we need to convert it to string
	scarb build
	jq '.abi = ( .abi | tostring )' target/dev/universal_deployer_UniversalDeployer.contract_class.json > tmp.json 
	mv tmp.json target/dev/universal_deployer_UniversalDeployer.contract_class.json
