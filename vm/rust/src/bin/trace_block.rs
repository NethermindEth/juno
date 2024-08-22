use std::time::{Duration, Instant};

use clap::Parser;
use starknet_core::types::BlockId;
use starknet_providers::{jsonrpc::HttpTransport, JsonRpcClient, Provider, Url};
use tokio;

#[derive(Parser, Debug)]
struct Args {
    block: u64,
    #[arg(short, long, default_value_t = str::to_string("http://localhost:6060/"))]
    url: String,
}

#[tokio::main(flavor = "current_thread")]
async fn main() {
    let args = Args::parse();

    let client = JsonRpcClient::new(HttpTransport::new(Url::parse(&args.url).unwrap()));

    let block_id = BlockId::Number(args.block);

    loop {
        let start = Instant::now();
        match client.trace_block_transactions(block_id).await {
            Ok(_a) => {
                let elapsed = start.elapsed();
                println!("finished in {} ", elapsed.as_secs());
                break;
            }
            Err(_) => {
                println!("Trying to connect");
                tokio::time::sleep(Duration::from_millis(500)).await;
                continue;
            }
        };
    }
}
