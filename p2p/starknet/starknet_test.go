package starknet_test

// func nopCloser() error { return nil }
//
// func TestClientHandler(t *testing.T) { //nolint:gocyclo
//	mockCtrl := gomock.NewController(t)
//	t.Cleanup(mockCtrl.Finish)
//
//	testNetwork := utils.Integration
//	testCtx, cancel := context.WithCancel(context.Background())
//	t.Cleanup(cancel)
//
//	mockNet, err := mocknet.FullMeshConnected(2)
//	require.NoError(t, err)
//
//	peers := mockNet.Peers()
//	require.Len(t, peers, 2)
//	handlerID := peers[0]
//	clientID := peers[1]
//
//	log, err := utils.NewZapLogger(utils.ERROR, false)
//	require.NoError(t, err)
//	mockReader := mocks.NewMockReader(mockCtrl)
//	handler := starknet.NewHandler(mockReader, log)
//
//	handlerHost := mockNet.Host(handlerID)
//	handlerHost.SetStreamHandler(starknet.CurrentBlockHeaderPID(testNetwork), handler.CurrentBlockHeaderHandler)
//	handlerHost.SetStreamHandler(starknet.BlockHeadersPID(&testNetwork), handler.BlockHeadersHandler)
//	handlerHost.SetStreamHandler(starknet.BlockBodiesPID(&testNetwork), handler.BlockBodiesHandler)
//	handlerHost.SetStreamHandler(starknet.EventsPID(&testNetwork), handler.EventsHandler)
//	handlerHost.SetStreamHandler(starknet.ReceiptsPID(&testNetwork), handler.ReceiptsHandler)
//	handlerHost.SetStreamHandler(starknet.TransactionsPID(&testNetwork), handler.TransactionsHandler)
//
//	clientHost := mockNet.Host(clientID)
//	client := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
//		return clientHost.NewStream(ctx, handlerID, pids...)
//	}, &testNetwork, log)
//
//	t.Run("get block headers", func(t *testing.T) {
//		type pair struct {
//			header      *core.Header
//			commitments *core.BlockCommitments
//		}
//		pairsPerBlock := []pair{}
//		for i := uint64(0); i < 2; i++ {
//			pairsPerBlock = append(pairsPerBlock, pair{
//				header: fillFelts(t, &core.Header{
//					Number:           i,
//					Timestamp:        i,
//					TransactionCount: i,
//					EventCount:       i,
//				}),
//				commitments: fillFelts(t, &core.BlockCommitments{}),
//			})
//		}
//
//		for blockNumber, pair := range pairsPerBlock {
//			blockNumber := uint64(blockNumber)
//			mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(pair.header, nil)
//			mockReader.EXPECT().BlockCommitmentsByNumber(blockNumber).Return(pair.commitments, nil)
//		}
//
//		numOfBlocks := uint64(len(pairsPerBlock))
//		res, cErr := client.RequestBlockHeaders(testCtx, &spec.BlockHeadersRequest{
//			Iteration: &spec.Iteration{
//				Start: &spec.Iteration_BlockNumber{
//					BlockNumber: 0,
//				},
//				Direction: spec.Iteration_Forward,
//				Limit:     numOfBlocks,
//				Step:      1,
//			},
//		})
//		require.NoError(t, cErr)
//
//		var count uint64
//		for response, valid := res(); valid; response, valid = res() {
//			if count == numOfBlocks {
//				assert.True(t, proto.Equal(&spec.Fin{}, response.Part[0].GetFin()))
//				count++
//				break
//			}
//
//			expectedPair := pairsPerBlock[count]
//			expectedResponse := expectedHeaderResponse(expectedPair.header, expectedPair.commitments)
//			assert.True(t, proto.Equal(expectedResponse, response))
//
//			assert.Equal(t, count, response.Part[0].GetHeader().Number)
//			count++
//		}
//
//		expectedCount := numOfBlocks + 1 // plus fin
//		require.Equal(t, expectedCount, count)
//
//		t.Run("get current block header", func(t *testing.T) {
//			headerAndCommitments := pairsPerBlock[0]
//			mockReader.EXPECT().Height().Return(headerAndCommitments.header.Number, nil)
//			mockReader.EXPECT().BlockHeaderByNumber(headerAndCommitments.header.Number).Return(headerAndCommitments.header, nil)
//			mockReader.EXPECT().BlockCommitmentsByNumber(headerAndCommitments.header.Number).Return(headerAndCommitments.commitments, nil)
//
//			res, cErr := client.RequestCurrentBlockHeader(testCtx, &spec.CurrentBlockHeaderRequest{})
//			require.NoError(t, cErr)
//
//			count, numOfBlocks = 0, 1
//			for response, valid := res(); valid; response, valid = res() {
//				if count == numOfBlocks {
//					assert.True(t, proto.Equal(&spec.Fin{}, response.Part[0].GetFin()))
//					count++
//					break
//				}
//
//				expectedPair := headerAndCommitments
//				expectedResponse := expectedHeaderResponse(expectedPair.header, expectedPair.commitments)
//				assert.True(t, proto.Equal(expectedResponse, response))
//
//				assert.Equal(t, count, response.Part[0].GetHeader().Number)
//				count++
//			}
//			expectedCount := numOfBlocks + 1 // plus fin
//			require.Equal(t, expectedCount, count)
//		})
//	})
//
//	t.Run("get block bodies", func(t *testing.T) {
//		/*
//			deployedClassHash := utils.HexToFelt(t, "0XCAFEBABE")
//			deployedAddress := utils.HexToFelt(t, "0XDEADBEEF")
//			replacedClassHash := utils.HexToFelt(t, "0XABCD")
//			replacedAddress := utils.HexToFelt(t, "0XABCDE")
//			declaredV0ClassAddr := randFelt(t)
//			declaredV0ClassHash := randFelt(t)
//			storageDiff := core.StorageDiff{
//				Key:   randFelt(t),
//				Value: randFelt(t),
//			}
//			const (
//				cairo0Program = "cairo_0_program"
//				cairo1Program = "cairo_1_program"
//			)
//			cairo1Class := &core.Cairo1Class{
//				Abi:     "cairo1 class abi",
//				AbiHash: randFelt(t),
//				EntryPoints: struct {
//					Constructor []core.SierraEntryPoint
//					External    []core.SierraEntryPoint
//					L1Handler   []core.SierraEntryPoint
//				}{},
//				Program:         feltSlice(2),
//				ProgramHash:     randFelt(t),
//				SemanticVersion: "1",
//				Compiled:        json.RawMessage(cairo1Program),
//			}
//
//			cairo0Class := &core.Cairo0Class{
//				Abi:     json.RawMessage("cairo0 class abi"),
//				Program: cairo1Program,
//			}
//
//			blocks := []struct {
//				number    uint64
//				stateDiff *core.StateDiff
//			}{
//				{
//					number: 0,
//					stateDiff: &core.StateDiff{
//						StorageDiffs: map[felt.Felt][]core.StorageDiff{
//							*deployedAddress: {
//								storageDiff,
//							},
//						},
//						Nonces: map[felt.Felt]*felt.Felt{
//							*deployedAddress: randFelt(t),
//							*replacedAddress: randFelt(t),
//						},
//						DeployedContracts: []core.AddressClassHashPair{
//							{
//								Address:   deployedAddress,
//								ClassHash: deployedClassHash,
//							},
//						},
//						DeclaredV0Classes: []*felt.Felt{declaredV0ClassAddr},
//						DeclaredV1Classes: []core.DeclaredV1Class{
//							{
//								ClassHash:         randFelt(t),
//								CompiledClassHash: randFelt(t),
//							},
//						},
//						ReplacedClasses: []core.AddressClassHashPair{
//							{
//								Address:   replacedAddress,
//								ClassHash: replacedClassHash,
//							},
//						},
//					},
//				},
//				{
//					number: 1,
//					stateDiff: &core.StateDiff{ // State Diff with a class declared and deployed in the same block
//						StorageDiffs: map[felt.Felt][]core.StorageDiff{
//							*deployedAddress: {
//								storageDiff,
//							},
//						},
//						Nonces: map[felt.Felt]*felt.Felt{
//							*deployedAddress: randFelt(t),
//							*replacedAddress: randFelt(t),
//						},
//						DeployedContracts: []core.AddressClassHashPair{
//							{
//								Address:   deployedAddress,
//								ClassHash: deployedClassHash,
//							},
//							{
//								Address:   declaredV0ClassAddr,
//								ClassHash: declaredV0ClassHash,
//							},
//						},
//						DeclaredV0Classes: []*felt.Felt{declaredV0ClassHash},
//						DeclaredV1Classes: []core.DeclaredV1Class{
//							{
//								ClassHash:         randFelt(t),
//								CompiledClassHash: randFelt(t),
//							},
//						},
//						ReplacedClasses: []core.AddressClassHashPair{
//							{
//								Address:   replacedAddress,
//								ClassHash: replacedClassHash,
//							},
//						},
//					},
//				},
//			}
//			limit := uint64(len(blocks))
//
//			for _, block := range blocks {
//				mockReader.EXPECT().BlockHeaderByNumber(block.number).Return(&core.Header{
//					Number: block.number,
//				}, nil)
//
//				mockReader.EXPECT().StateUpdateByNumber(block.number).Return(&core.StateUpdate{
//					StateDiff: block.stateDiff,
//				}, nil)
//
//				stateHistory := mocks.NewMockStateHistoryReader(mockCtrl)
//				v0Class := block.stateDiff.DeclaredV0Classes[0]
//				stateHistory.EXPECT().Class(v0Class).Return(&core.DeclaredClass{
//					At:    block.number,
//					Class: cairo0Class,
//				}, nil)
//				v1Class := block.stateDiff.DeclaredV1Classes[0]
//				stateHistory.EXPECT().Class(v1Class.ClassHash).Return(&core.DeclaredClass{
//					At:    block.number,
//					Class: cairo1Class,
//				}, nil)
//
//				stateHistory.EXPECT().ContractClassHash(deployedAddress).Return(deployedClassHash, nil).AnyTimes()
//				stateHistory.EXPECT().ContractClassHash(replacedAddress).Return(replacedClassHash, nil).AnyTimes()
//
//				mockReader.EXPECT().StateAtBlockNumber(block.number).Return(stateHistory, nopCloser, nil)
//			}
//
//			res, cErr := client.RequestBlockBodies(testCtx, &spec.BlockBodiesRequest{
//				Iteration: &spec.Iteration{
//					Start: &spec.Iteration_BlockNumber{
//						BlockNumber: blocks[0].number,
//					},
//					Direction: spec.Iteration_Forward,
//					Limit:     limit,
//					Step:      1,
//				},
//			})
//			require.NoError(t, cErr)
//
//			var expectedMessages []*spec.BlockBodiesResponse
//
//			for _, b := range blocks {
//				expectedMessages = append(expectedMessages, []*spec.BlockBodiesResponse{
//					{
//						Id: &spec.BlockID{
//							Number: b.number,
//						},
//						BodyMessage: &spec.BlockBodiesResponse_Diff{
//							Diff: &spec.StateDiff{
//								ContractDiffs: []*spec.StateDiff_ContractDiff{
//									{
//										Address:   core2p2p.AdaptAddress(deployedAddress),
//										ClassHash: core2p2p.AdaptFelt(deployedClassHash),
//										Nonce:     core2p2p.AdaptFelt(b.stateDiff.Nonces[*deployedAddress]),
//										Values: []*spec.ContractStoredValue{
//											{
//												Key:   core2p2p.AdaptFelt(storageDiff.Key),
//												Value: core2p2p.AdaptFelt(storageDiff.Value),
//											},
//										},
//									},
//									{
//										Address:   core2p2p.AdaptAddress(replacedAddress),
//										ClassHash: core2p2p.AdaptFelt(replacedClassHash),
//										Nonce:     core2p2p.AdaptFelt(b.stateDiff.Nonces[*replacedAddress]),
//									},
//								},
//								ReplacedClasses:   utils.Map(b.stateDiff.ReplacedClasses, core2p2p.AdaptAddressClassHashPair),
//								DeployedContracts: utils.Map(b.stateDiff.DeployedContracts, core2p2p.AdaptAddressClassHashPair),
//							},
//						},
//					},
//					{
//						Id: &spec.BlockID{
//							Number: b.number,
//						},
//						BodyMessage: &spec.BlockBodiesResponse_Classes{
//							Classes: &spec.Classes{
//								Domain:  0,
//								Classes: []*spec.Class{core2p2p.AdaptClass(cairo0Class), core2p2p.AdaptClass(cairo1Class)},
//							},
//						},
//					},
//					{
//						Id: &spec.BlockID{
//							Number: b.number,
//						},
//						BodyMessage: &spec.BlockBodiesResponse_Proof{
//							Proof: &spec.BlockProof{
//								Proof: nil,
//							},
//						},
//					},
//					{
//						Id: &spec.BlockID{
//							Number: b.number,
//						},
//						BodyMessage: &spec.BlockBodiesResponse_Fin{},
//					},
//				}...)
//			}
//
//			expectedMessages = append(expectedMessages, &spec.BlockBodiesResponse{
//				Id:          nil,
//				BodyMessage: &spec.BlockBodiesResponse_Fin{},
//			})
//
//			var count int
//			for body, valid := res(); valid; body, valid = res() {
//				if bodyProof, ok := body.BodyMessage.(*spec.BlockBodiesResponse_Proof); ok {
//					// client generates random slice of bytes in proofs for now
//					bodyProof.Proof = nil
//				}
//
//				if count == 0 || count == 4 {
//					diff := body.BodyMessage.(*spec.BlockBodiesResponse_Diff).Diff.ContractDiffs
//					sortContractDiff(diff)
//
//					expectedDiff := expectedMessages[count].BodyMessage.(*spec.BlockBodiesResponse_Diff).Diff.ContractDiffs
//					sortContractDiff(expectedDiff)
//				}
//
//				if !assert.True(t, proto.Equal(expectedMessages[count], body), "iteration %d, type %T", count, body.BodyMessage) {
//					spew.Dump(body.BodyMessage)
//					spew.Dump(expectedMessages[count])
//				}
//				count++
//			}
//			require.Equal(t, len(expectedMessages), count)
//		*/
//	})
//
//	t.Run("get receipts", func(t *testing.T) {
//		txH := randFelt(t)
//		// There are common receipt fields shared by all of different transactions.
//		commonReceipt := &core.TransactionReceipt{
//			TransactionHash: txH,
//			Fee:             randFelt(t),
//			L2ToL1Message:   []*core.L2ToL1Message{fillFelts(t, &core.L2ToL1Message{}), fillFelts(t, &core.L2ToL1Message{})},
//			ExecutionResources: &core.ExecutionResources{
//				BuiltinInstanceCounter: core.BuiltinInstanceCounter{
//					Pedersen:   1,
//					RangeCheck: 2,
//					Bitwise:    3,
//					Output:     4,
//					Ecsda:      5,
//					EcOp:       6,
//					Keccak:     7,
//					Poseidon:   8,
//				},
//				MemoryHoles: 9,
//				Steps:       10,
//			},
//			RevertReason:  "some revert reason",
//			Events:        []*core.Event{fillFelts(t, &core.Event{}), fillFelts(t, &core.Event{})},
//			L1ToL2Message: fillFelts(t, &core.L1ToL2Message{}),
//		}
//
//		specReceiptCommon := &spec.Receipt_Common{
//			TransactionHash:    core2p2p.AdaptHash(commonReceipt.TransactionHash),
//			ActualFee:          core2p2p.AdaptFelt(commonReceipt.Fee),
//			MessagesSent:       utils.Map(commonReceipt.L2ToL1Message, core2p2p.AdaptMessageToL1),
//			ExecutionResources: core2p2p.AdaptExecutionResources(commonReceipt.ExecutionResources),
//			RevertReason:       commonReceipt.RevertReason,
//		}
//
//		invokeTx := &core.InvokeTransaction{TransactionHash: txH}
//		expectedInvoke := &spec.Receipt{
//			Type: &spec.Receipt_Invoke_{
//				Invoke: &spec.Receipt_Invoke{
//					Common: specReceiptCommon,
//				},
//			},
//		}
//
//		declareTx := &core.DeclareTransaction{TransactionHash: txH}
//		expectedDeclare := &spec.Receipt{
//			Type: &spec.Receipt_Declare_{
//				Declare: &spec.Receipt_Declare{
//					Common: specReceiptCommon,
//				},
//			},
//		}
//
//		l1Txn := &core.L1HandlerTransaction{
//			TransactionHash:    txH,
//			CallData:           []*felt.Felt{new(felt.Felt).SetBytes([]byte("calldata 1")), new(felt.Felt).SetBytes([]byte("calldata 2"))},
//			ContractAddress:    new(felt.Felt).SetBytes([]byte("contract address")),
//			EntryPointSelector: new(felt.Felt).SetBytes([]byte("entry point selector")),
//			Nonce:              new(felt.Felt).SetBytes([]byte("nonce")),
//		}
//		expectedL1Handler := &spec.Receipt{
//			Type: &spec.Receipt_L1Handler_{
//				L1Handler: &spec.Receipt_L1Handler{
//					Common:  specReceiptCommon,
//					MsgHash: &spec.Hash{Elements: l1Txn.MessageHash()},
//				},
//			},
//		}
//
//		deployAccTxn := &core.DeployAccountTransaction{
//			DeployTransaction: core.DeployTransaction{
//				TransactionHash: txH,
//				ContractAddress: new(felt.Felt).SetBytes([]byte("contract address")),
//			},
//		}
//		expectedDeployAccount := &spec.Receipt{
//			Type: &spec.Receipt_DeployAccount_{
//				DeployAccount: &spec.Receipt_DeployAccount{
//					Common:          specReceiptCommon,
//					ContractAddress: core2p2p.AdaptFelt(deployAccTxn.ContractAddress),
//				},
//			},
//		}
//
//		deployTxn := &core.DeployTransaction{
//			TransactionHash: txH,
//			ContractAddress: new(felt.Felt).SetBytes([]byte("contract address")),
//		}
//		expectedDeploy := &spec.Receipt{
//			Type: &spec.Receipt_DeprecatedDeploy{
//				DeprecatedDeploy: &spec.Receipt_Deploy{
//					Common:          specReceiptCommon,
//					ContractAddress: core2p2p.AdaptFelt(deployTxn.ContractAddress),
//				},
//			},
//		}
//
//		tests := []struct {
//			b          *core.Block
//			expectedRs *spec.Receipts
//		}{
//			{
//				b: &core.Block{
//					Header:       &core.Header{Number: 0, Hash: randFelt(t)},
//					Transactions: []core.Transaction{invokeTx},
//					Receipts:     []*core.TransactionReceipt{commonReceipt},
//				},
//				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedInvoke}},
//			},
//			{
//				b: &core.Block{
//					Header:       &core.Header{Number: 1, Hash: randFelt(t)},
//					Transactions: []core.Transaction{declareTx},
//					Receipts:     []*core.TransactionReceipt{commonReceipt},
//				},
//				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedDeclare}},
//			},
//			{
//				b: &core.Block{
//					Header:       &core.Header{Number: 2, Hash: randFelt(t)},
//					Transactions: []core.Transaction{l1Txn},
//					Receipts:     []*core.TransactionReceipt{commonReceipt},
//				},
//				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedL1Handler}},
//			},
//			{
//				b: &core.Block{
//					Header:       &core.Header{Number: 3, Hash: randFelt(t)},
//					Transactions: []core.Transaction{deployAccTxn},
//					Receipts:     []*core.TransactionReceipt{commonReceipt},
//				},
//				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedDeployAccount}},
//			},
//			{
//				b: &core.Block{
//					Header:       &core.Header{Number: 4, Hash: randFelt(t)},
//					Transactions: []core.Transaction{deployTxn},
//					Receipts:     []*core.TransactionReceipt{commonReceipt},
//				},
//				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedDeploy}},
//			},
//			{
//				// block with multiple txs receipts
//				b: &core.Block{
//					Header:       &core.Header{Number: 5, Hash: randFelt(t)},
//					Transactions: []core.Transaction{invokeTx, declareTx},
//					Receipts:     []*core.TransactionReceipt{commonReceipt, commonReceipt},
//				},
//				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedInvoke, expectedDeclare}},
//			},
//		}
//
//		numOfBs := uint64(len(tests))
//		for _, test := range tests {
//			mockReader.EXPECT().BlockByNumber(test.b.Number).Return(test.b, nil)
//		}
//
//		res, cErr := client.RequestReceipts(testCtx, &spec.ReceiptsRequest{Iteration: &spec.Iteration{
//			Start:     &spec.Iteration_BlockNumber{BlockNumber: tests[0].b.Number},
//			Direction: spec.Iteration_Forward,
//			Limit:     numOfBs,
//			Step:      1,
//		}})
//		require.NoError(t, cErr)
//
//		var count uint64
//		for receipts, valid := res(); valid; receipts, valid = res() {
//			if count == numOfBs {
//				assert.NotNil(t, receipts.GetFin())
//				continue
//			}
//
//			assert.Equal(t, count, receipts.Id.Number)
//
//			expectedRs := tests[count].expectedRs
//			assert.True(t, proto.Equal(expectedRs, receipts.GetReceipts()))
//			count++
//		}
//		require.Equal(t, numOfBs, count)
//	})
//
//	t.Run("get txns", func(t *testing.T) {
//		blocks := []*core.Block{
//			{
//				Header: &core.Header{
//					Number: 0,
//				},
//				Transactions: []core.Transaction{
//					fillFelts(t, &core.DeployTransaction{
//						ConstructorCallData: feltSlice(3),
//					}),
//					fillFelts(t, &core.L1HandlerTransaction{
//						CallData: feltSlice(2),
//						Version:  txVersion(1),
//					}),
//				},
//			},
//			{
//				Header: &core.Header{
//					Number: 1,
//				},
//				Transactions: []core.Transaction{
//					fillFelts(t, &core.DeployAccountTransaction{
//						DeployTransaction: core.DeployTransaction{
//							ConstructorCallData: feltSlice(3),
//							Version:             txVersion(1),
//						},
//						TransactionSignature: feltSlice(2),
//					}),
//				},
//			},
//			{
//				Header: &core.Header{
//					Number: 2,
//				},
//				Transactions: []core.Transaction{
//					fillFelts(t, &core.DeclareTransaction{
//						TransactionSignature: feltSlice(2),
//						Version:              txVersion(0),
//					}),
//					fillFelts(t, &core.DeclareTransaction{
//						TransactionSignature: feltSlice(2),
//						Version:              txVersion(1),
//					}),
//				},
//			},
//			{
//				Header: &core.Header{
//					Number: 3,
//				},
//				Transactions: []core.Transaction{
//					fillFelts(t, &core.InvokeTransaction{
//						CallData:             feltSlice(3),
//						TransactionSignature: feltSlice(2),
//						Version:              txVersion(0),
//					}),
//					fillFelts(t, &core.InvokeTransaction{
//						CallData:             feltSlice(4),
//						TransactionSignature: feltSlice(2),
//						Version:              txVersion(1),
//					}),
//				},
//			},
//		}
//		numOfBlocks := uint64(len(blocks))
//
//		for _, block := range blocks {
//			mockReader.EXPECT().BlockByNumber(block.Number).Return(block, nil)
//		}
//
//		res, cErr := client.RequestTransactions(testCtx, &spec.TransactionsRequest{
//			Iteration: &spec.Iteration{
//				Start: &spec.Iteration_BlockNumber{
//					BlockNumber: blocks[0].Number,
//				},
//				Direction: spec.Iteration_Forward,
//				Limit:     numOfBlocks,
//				Step:      1,
//			},
//		})
//		require.NoError(t, cErr)
//
//		var count uint64
//		for txn, valid := res(); valid; txn, valid = res() {
//			if count == numOfBlocks {
//				assert.NotNil(t, txn.GetFin())
//				break
//			}
//
//			assert.Equal(t, count, txn.Id.Number)
//
//			expectedTx := mapToExpectedTransactions(blocks[count])
//			assert.True(t, proto.Equal(expectedTx, txn.GetTransactions()))
//			count++
//		}
//		require.Equal(t, numOfBlocks, count)
//	})
//
//	t.Run("get events", func(t *testing.T) {
//		eventsPerBlock := [][]*core.Event{
//			{}, // block with no events
//			{
//				{
//					From: randFelt(t),
//					Data: feltSlice(1),
//					Keys: feltSlice(1),
//				},
//			},
//			{
//				{
//					From: randFelt(t),
//					Data: feltSlice(2),
//					Keys: feltSlice(2),
//				},
//				{
//					From: randFelt(t),
//					Data: feltSlice(3),
//					Keys: feltSlice(3),
//				},
//			},
//		}
//		for blockNumber, events := range eventsPerBlock {
//			blockNumber := uint64(blockNumber)
//			mockReader.EXPECT().BlockByNumber(blockNumber).Return(&core.Block{
//				Header: &core.Header{
//					Number: blockNumber,
//				},
//				Receipts: []*core.TransactionReceipt{
//					{
//						TransactionHash: new(felt.Felt).SetUint64(blockNumber),
//						Events:          events,
//					},
//				},
//			}, nil)
//		}
//
//		numOfBlocks := uint64(len(eventsPerBlock))
//		res, cErr := client.RequestEvents(testCtx, &spec.EventsRequest{
//			Iteration: &spec.Iteration{
//				Start: &spec.Iteration_BlockNumber{
//					BlockNumber: 0,
//				},
//				Direction: spec.Iteration_Forward,
//				Limit:     numOfBlocks,
//				Step:      1,
//			},
//		})
//		require.NoError(t, cErr)
//
//		var count uint64
//		for evnt, valid := res(); valid; evnt, valid = res() {
//			if count == numOfBlocks {
//				assert.True(t, proto.Equal(&spec.Fin{}, evnt.GetFin()))
//				count++
//				break
//			}
//
//			assert.Equal(t, count, evnt.Id.Number)
//
//			passedEvents := eventsPerBlock[int(count)]
//			expectedEventsResponse := &spec.EventsResponse_Events{
//				Events: &spec.Events{
//					Items: utils.Map(passedEvents, func(e *core.Event) *spec.Event {
//						return core2p2p.AdaptEvent(e, new(felt.Felt).SetUint64(count))
//					}),
//				},
//			}
//
//			assert.True(t, proto.Equal(expectedEventsResponse.Events, evnt.GetEvents()))
//			count++
//		}
//		expectedCount := numOfBlocks + 1 // numOfBlocks messages with blocks + 1 fin message
//		require.Equal(t, expectedCount, count)
//
//		t.Run("block with multiple tx", func(t *testing.T) {
//			blockNumber := uint64(0)
//			mockReader.EXPECT().BlockByNumber(blockNumber).Return(&core.Block{
//				Header: &core.Header{
//					Number: blockNumber,
//				},
//				Receipts: []*core.TransactionReceipt{
//					{
//						TransactionHash: new(felt.Felt).SetUint64(0),
//						Events:          eventsPerBlock[0],
//					},
//					{
//						TransactionHash: new(felt.Felt).SetUint64(1),
//						Events:          eventsPerBlock[1],
//					},
//					{
//						TransactionHash: new(felt.Felt).SetUint64(2),
//						Events:          eventsPerBlock[2],
//					},
//				},
//			}, nil)
//
//			res, cErr = client.RequestEvents(testCtx, &spec.EventsRequest{
//				Iteration: &spec.Iteration{
//					Start: &spec.Iteration_BlockNumber{
//						BlockNumber: blockNumber,
//					},
//					Direction: spec.Iteration_Forward,
//					Limit:     1,
//					Step:      1,
//				},
//			})
//
//			expectedEventsResponse := &spec.EventsResponse_Events{
//				Events: &spec.Events{
//					Items: []*spec.Event{
//						core2p2p.AdaptEvent(eventsPerBlock[1][0], new(felt.Felt).SetUint64(1)),
//						core2p2p.AdaptEvent(eventsPerBlock[2][0], new(felt.Felt).SetUint64(2)),
//						core2p2p.AdaptEvent(eventsPerBlock[2][1], new(felt.Felt).SetUint64(2)),
//					},
//				},
//			}
//			count = 0
//			for evnt, valid := res(); valid; evnt, valid = res() {
//				if count == 1 {
//					assert.True(t, proto.Equal(&spec.Fin{}, evnt.GetFin()))
//					break
//				}
//
//				assert.Equal(t, count, evnt.Id.Number)
//
//				assert.True(t, proto.Equal(expectedEventsResponse.Events, evnt.GetEvents()))
//				count++
//			}
//			require.NoError(t, cErr)
//		})
//	})
//}
//
// func expectedHeaderResponse(h *core.Header, c *core.BlockCommitments) *spec.BlockHeadersResponse {
//	adaptHash := core2p2p.AdaptHash
//	return &spec.BlockHeadersResponse{
//		Part: []*spec.BlockHeadersResponsePart{
//			{
//				HeaderMessage: &spec.BlockHeadersResponsePart_Header{
//					Header: &spec.BlockHeader{
//						ParentHash:       adaptHash(h.ParentHash),
//						Number:           h.Number,
//						Time:             timestamppb.New(time.Unix(int64(h.Timestamp), 0)),
//						SequencerAddress: core2p2p.AdaptAddress(h.SequencerAddress),
//						State: &spec.Patricia{
//							Height: 251,
//							Root:   adaptHash(h.GlobalStateRoot),
//						},
//						Transactions: &spec.Merkle{
//							NLeaves: uint32(h.TransactionCount),
//							Root:    adaptHash(c.TransactionCommitment),
//						},
//						Events: &spec.Merkle{
//							NLeaves: uint32(h.EventCount),
//							Root:    adaptHash(c.EventCommitment),
//						},
//					},
//				},
//			},
//			{
//				HeaderMessage: &spec.BlockHeadersResponsePart_Signatures{
//					Signatures: &spec.Signatures{
//						Block:      core2p2p.AdaptBlockID(h),
//						Signatures: utils.Map(h.Signatures, core2p2p.AdaptSignature),
//					},
//				},
//			},
//		},
//	}
//}
//
// func mapToExpectedTransactions(block *core.Block) *spec.Transactions {
//	return &spec.Transactions{
//		Items: utils.Map(block.Transactions, core2p2p.AdaptTransaction),
//	}
//}
//
// func txVersion(v uint64) *core.TransactionVersion {
//	var f felt.Felt
//	f.SetUint64(v)
//
//	txV := core.TransactionVersion(f)
//	return &txV
//}
//
// func feltSlice(n int) []*felt.Felt {
//	return make([]*felt.Felt, n)
//}
//
// func randFelt(t *testing.T) *felt.Felt {
//	t.Helper()
//
//	f, err := new(felt.Felt).SetRandom()
//	require.NoError(t, err)
//
//	return f
//}
//
// func fillFelts[T any](t *testing.T, i T) T {
//	v := reflect.ValueOf(i)
//	if v.Kind() == reflect.Ptr && !v.IsNil() {
//		v = v.Elem()
//	}
//	typ := v.Type()
//
//	const feltTypeStr = "*felt.Felt"
//
//	for i := 0; i < v.NumField(); i++ {
//		f := v.Field(i)
//		ftyp := typ.Field(i).Type // Get the type of the current field
//
//		// Skip unexported fields
//		if !f.CanSet() {
//			continue
//		}
//
//		switch f.Kind() {
//		case reflect.Ptr:
//			// Check if the type is Felt
//			if ftyp.String() == feltTypeStr {
//				f.Set(reflect.ValueOf(randFelt(t)))
//			} else if f.IsNil() {
//				// Initialise the pointer if it's nil
//				f.Set(reflect.New(ftyp.Elem()))
//			}
//
//			if f.Elem().Kind() == reflect.Struct {
//				// Recursive call for nested structs
//				fillFelts(t, f.Interface())
//			}
//		case reflect.Slice:
//			// For slices, loop and populate
//			for j := 0; j < f.Len(); j++ {
//				elem := f.Index(j)
//				if elem.Type().String() == feltTypeStr {
//					elem.Set(reflect.ValueOf(randFelt(t)))
//				}
//			}
//		case reflect.Struct:
//			// Recursive call for nested structs
//			fillFelts(t, f.Addr().Interface())
//		}
//	}
//
//	return i
//}
//
// func sortContractDiff(diff []*spec.StateDiff_ContractDiff) {
//	sort.Slice(diff, func(i, j int) bool {
//		iAddress := diff[i].Address
//		jAddress := diff[j].Address
//		return bytes.Compare(iAddress.Elements, jAddress.Elements) < 0
//	})
//}
//
// func noError[T any](t *testing.T, f func() (T, error)) T {
//	t.Helper()
//
//	v, err := f()
//	require.NoError(t, err)
//
//	return v
//}
