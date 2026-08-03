package node

// note(rdr): sequencer code that was cluttering the node.go initialization

// func initalizeSequencer(
// 	cfg *Config,
// 	database db.KeyValueStore,
// 	version string,
// 	chain *blockchain.Blockchain,
// 	compiler *ThrottledCompiler,
// 	logger log.StructuredLogger,
// ) (*sequencer.Sequencer, error) {
// 	logger.Warn(
// 		"Sequencer features enabled. Please note the sequencer is in experimental stage",
// 	)
//
// 	// Sequencer mode only supports known networks and
// 	// uses default fee tokens (custom networks not supported yet)
// 	if !slices.Contains(networks.KnownNetworkNames, cfg.Network.Name) {
// 		return nil, fmt.Errorf("custom networks are not yet supported in sequencer mode")
// 	}
// 	pKey, kErr := ecdsa.GenerateKey(rand.Reader)
// 	if kErr != nil {
// 		return nil, kErr
// 	}
//
// 	feeTokens := networks.DefaultFeeTokenAddresses
// 	chainInfo := vm.ChainInfo{
// 		ChainID:           cfg.Network.L2ChainID,
// 		FeeTokenAddresses: feeTokens,
// 	}
// 	nodeVM := vm.New(&chainInfo, false, logger)
//
// 	const mempoolLimit = 1024
// 	mempool := mempool.New(database, chain, mempoolLimit, logger)
// 	executor := builder.NewExecutor(chain, nodeVM, logger, cfg.SeqDisableFees, false)
// 	builder := builder.New(chain, executor)
// 	seq := sequencer.New(
// 		&builder,
// 		mempool,
// 		felt.NewFromUint64[felt.Felt](1234),
// 		pKey,
// 		time.Second*time.Duration(cfg.SeqBlockTime),
// 		logger,
// 	)
//
// 	return &seq, nil
//
// 	// other sequencer configurations
//
// 	// throttledVM := NewThrottledVM(nodeVM, cfg.MaxVMs, uint64(cfg.MaxVMQueue))
// 	// rpcHandler := rpc.New(chain, &seq, throttledVM, version, logger, &cfg.Network).
// 	//
// 	//	WithCompiler(compiler).
// 	//	WithMempool(mempool).
// 	//	WithCallMaxSteps(cfg.RPCCallMaxSteps).
// 	//	WithCallMaxGas(cfg.RPCCallMaxGas)
// 	//
// 	// services = append(services, &seq)
// 	//
// 	//	if cfg.Prune {
// 	//		prunerOpts := make([]pruner.Option, 0, 2)
// 	//		if cfg.Metrics {
// 	//			prunerOpts = append(prunerOpts, pruner.WithListener(makePrunerMetrics()))
// 	//		}
// 	//
// 	//		prunerOpts = append(prunerOpts, pruner.WithMinAge(cfg.PruneMinAge))
// 	//		p := pruner.New(
// 	//			database,
// 	//			cfg.RetainedBlocks,
// 	//			seq.SubscribeNewHeads().Subscription,
// 	//			chain.SubscribeL1Head().Subscription,
// 	//			logger,
// 	//			prunerOpts...,
// 	//		)
// 	//		services = append(services, p)
// 	//	}
// }

// When running the node the following code should be executed

//	if n.cfg.Sequencer {
// 		feeTokens := networks.DefaultFeeTokenAddresses
// 		chainInfo := vm.ChainInfo{
// 			ChainID:           n.cfg.Network.L2ChainID,
// 			FeeTokenAddresses: feeTokens,
// 		}
//
// 		err := buildGenesis(
// 			ctx,
// 			n.cfg.SeqGenesisFile,
// 			n.blockchain,
// 			vm.New(&chainInfo, false, n.logger),
// 			n.cfg.RPCCallMaxSteps,
// 			n.cfg.RPCCallMaxGas,
// 			n.cfg.NewState,
// 			n.compiler,
// 		)
// 		if err != nil {
// 			n.logger.Error("Error building genesis state", zap.Error(err))
// 			return
// 		}
// 	}
//

// func buildGenesis(
// 	ctx context.Context,
// 	genesisPath string,
// 	bc *blockchain.Blockchain,
// 	v vm.VM,
// 	maxSteps uint64,
// 	maxGas uint64,
// 	useNewState bool,
// 	compiler compiler.Compiler,
// ) error {
// 	if _, err := bc.Height(); !errors.Is(err, db.ErrKeyNotFound) {
// 		return err
// 	}
//
// 	var diff core.StateDiff
// 	var classes map[felt.Felt]core.ClassDefinition
// 	switch {
// 	case genesisPath != "":
// 		genesisConfig, err := genesis.Read(genesisPath)
// 		if err != nil {
// 			return err
// 		}
//
// 		diff, classes, err = genesis.GenesisStateDiff(
// 			ctx,
// 			genesisConfig,
// 			v,
// 			bc.Network(),
// 			maxSteps,
// 			maxGas,
// 			useNewState,
// 			compiler,
// 		)
// 		if err != nil {
// 			return err
// 		}
//
// 	default:
// 		diff = core.EmptyStateDiff()
// 	}
//
// 	return bc.StoreGenesis(&diff, classes)
// }
