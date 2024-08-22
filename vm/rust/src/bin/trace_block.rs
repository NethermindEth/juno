use std::time::{Duration, Instant};

use starknet_core::types::BlockId;
use starknet_providers::{jsonrpc::HttpTransport, JsonRpcClient, Provider, Url};
use tokio;

#[tokio::main(flavor = "current_thread")]
async fn main() {
    let client = JsonRpcClient::new(HttpTransport::new(
        Url::parse("http://localhost:6060/").unwrap(),
    ));

    let block_id = BlockId::Number(633_333);

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
