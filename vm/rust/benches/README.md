To record all Juno calls for a block:
- Start Juno with your preferred options and prepend (or set the) environment variable `JUNO_RECORD_DIR`. 
    - `JUNO_RECORD_DIR=./record_mainnet ./build/juno OPTIONS`  
    - If the directory doesn't exist it will be created. 
    - This will record the Juno calls for native under JUNO_RECORD_DIR/Native.
- To record the Juno calls for the VM is the same as the last step and prepend `JUNO_EXECUTOR=VM`
    - `JUNO_EXECUTOR=VM JUNO_RECORD_DIR=./record_mainnet ./build/juno OPTIONS`  
    - This will record the Juno calls for VM under JUNO_RECORD_DIR/VM.
- Use the traceblock ulility to trace a block
    - `cargo r --bin traceblock -- BLOCK_NUMBER` 
 
To replay both the VM and Native recorded transaction in a benchmark:
- `JUNO_RECORD_DIR=./record_mainnet BENCH_BLOCK=BLOCK_NUMBER cargo bench`

 To profile a benchmark with samply:
- Get [samply](https://github.com/mstange/samply) for profiling
- Filter the benchmark to the one you want to run. See `blockifier-perf` for the possible benchmarks and the filter remark below.
- Use `--profile-time` to skip analysis and storing, and run the benchmark for a set amount of time expressed in seconds. The benchmark will repeat until the timer has been reached.  
- And for less noise change `config = Criterion::default().with_profiler(PProfProfiler::new(10, Output::Protobuf));` to `config = Criterion::default()`. 
- `JUNO_RECORD_DIR=./record_mainnet samply record cargo bench -- preload --profile-time 120`

Even when filtering the setup of every benchmark will still be executed. For example only running the bench for `loading` will still execute the setup of `preload`. 



