use std::time::{Duration, Instant};

use clap::Parser;
use starknet_core::types::BlockId;
use starknet_providers::{jsonrpc::HttpTransport, JsonRpcClient, Provider, Url};
use tokio::{self, task::JoinSet};

#[derive(Parser, Debug)]
struct Args {
    blocks: Vec<u64>,
    #[arg(short, long, default_value_t = str::to_string("http://localhost:6060/"))]
    url: String,
    #[arg(short, long, default_value_t = 2)]
    max_concurrent: usize,
}

async fn trace_block(url: Url, block_id: BlockId) {
    let client: JsonRpcClient<HttpTransport> = JsonRpcClient::new(HttpTransport::new(url));
    loop {
        let start = Instant::now();
        match client.trace_block_transactions(block_id).await {
            Ok(_a) => {
                let elapsed = start.elapsed();
                println!(
                    "Block {block_id:?} finished in {} seconds",
                    elapsed.as_secs()
                );
                break;
            }
            Err(_) => {
                println!("Trying to connect {block_id:?}");
                tokio::time::sleep(Duration::from_millis(500)).await;
                continue;
            }
        };
    }
}

#[tokio::main(flavor = "current_thread")]
async fn main() {
    let args = Args::parse();
    let url = Url::parse(&args.url).unwrap();

    let mut set = JoinSet::new();

    for block in args.blocks {
        let block_id = BlockId::Number(block);
        while set.len() >= args.max_concurrent {
            set.join_next().await;
        }
        set.spawn(trace_block(url.clone(), block_id));
    }

    while let Some(_output) = set.join_next().await {}
}
