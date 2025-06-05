import os, time, requests, datetime, json, sys, subprocess

# Configuration
target_block = int(os.environ.get("BLOCK_TARGET", "50000"))
rpc_url = "http://localhost:6060"
check_interval = 60  # seconds
history_file = "/var/lib/juno/benchmark-history.json"
start_time = datetime.datetime.now()

print(f"⏳ Benchmark started. Waiting for Juno RPC to become available...")

def push_to_github(result):
    """Simple git push to gh-pages"""
    deploy_key = os.environ.get("GH_DEPLOY_KEY")
    if not deploy_key:
        print("⚠️ No GitHub deploy key, skipping git push")
        return
    
    try:
        # SSH setup
        ssh_dir = "/root/.ssh"
        os.makedirs(ssh_dir, exist_ok=True)
        os.chmod(ssh_dir, 0o700)
        
        # Write SSH key
        key_path = os.path.join(ssh_dir, "id_rsa")
        with open(key_path, "wb") as f:
            f.write(deploy_key.strip().encode() + b"\n")
        os.chmod(key_path, 0o600)
        
        # Get GitHub's host key
        known_hosts = os.path.join(ssh_dir, "known_hosts")
        with open(known_hosts, "w") as f:
            subprocess.run(["ssh-keyscan", "github.com"], stdout=f, check=True)
        os.chmod(known_hosts, 0o644)
        
        # Git operations
        repo_dir = "/tmp/juno"
        subprocess.run(["git", "clone", "-b", "gh-pages", "--depth", "1", "git@github.com:NethermindEth/juno.git", repo_dir], check=True)
        
        # Set local git config
        subprocess.run(["git", "config", "user.email", "benchmark@nethermind.io"], cwd=repo_dir, check=True)
        subprocess.run(["git", "config", "user.name", "Juno Benchmark Bot"], cwd=repo_dir, check=True)
        
        # Update benchmark data
        benchmark_file = os.path.join(repo_dir, "benchmark-data.json")
        data = []
        if os.path.exists(benchmark_file):
            with open(benchmark_file, "r") as f:
                data = json.load(f)
        
        data.append(result)
        with open(benchmark_file, "w") as f:
            json.dump(data, f, indent=2)
        
        # Commit and push
        subprocess.run(["git", "add", "benchmark-data.json"], cwd=repo_dir, check=True)
        subprocess.run(["git", "commit", "-m", f"benchmark: {result['version']} block {result['block_target']}"], cwd=repo_dir, check=True)
        subprocess.run(["git", "push"], cwd=repo_dir, check=True)
        print("✅ Pushed to GitHub")
        
    except Exception as e:
        print(f"⚠️ Git push failed: {e}")

# Wait for Juno to start (with simple retry)
version = "unknown"
for attempt in range(1, 11):  # 10 attempts max
    try:
        r = requests.post(
            rpc_url,
            json={"jsonrpc": "2.0", "method": "juno_version", "params": [], "id": 0},
            timeout=5
        )
        version = r.json().get("result", "unknown")
        print(f"🆗 Juno version: {version}")
        break
    except Exception as e:
        print(f"⚠️ RPC not ready (attempt {attempt}/10): {e}")
        if attempt == 10:
            print("❌ Juno RPC not responding after 10 retries. Failing benchmark.")
            sys.exit(1)
        time.sleep(min(30, 5 * attempt))  # Increasing backoff, max 30s

print(f"[{version}] ⏱ Starting sync measurement to block {target_block}...")

# Monitor block progress
consecutive_errors = 0
while True:
    try:
        # Check current block
        r = requests.post(
            rpc_url,
            json={"jsonrpc": "2.0", "method": "starknet_blockNumber", "params": [], "id": 1},
            timeout=10
        )
        block = int(r.json()["result"])
        consecutive_errors = 0
        
        # Log progress
        print(f"[{version}] Current block: {block}/{target_block}")
        
        # Check if we've reached the target
        if block >= target_block:
            duration = datetime.datetime.now() - start_time
            print(f"[{version}] ✅ Sync completed in {duration}")
            
            # Record the benchmark results
            result = {
                "version": version,
                "block_target": target_block,
                "duration_seconds": duration.total_seconds(),
                "timestamp": datetime.datetime.now(datetime.UTC).isoformat()
            }
            
            print(f"[{version}] 📊 BENCHMARK RESULT: {json.dumps(result)}")
            
            # Save to history file
            try:
                history = []
                if os.path.exists(history_file):
                    try:
                        with open(history_file, "r") as f:
                            history = json.load(f)
                    except:
                        pass  # Start with empty history if file can't be read
                
                history.append(result)
                
                with open(history_file, "w") as f:
                    json.dump(history, f, indent=2)
                print(f"[{version}] Results saved to {history_file}")
            except Exception as e:
                print(f"[{version}] Warning: Could not save results: {e}")
            
            # Push results to GitHub
            push_to_github(result)
            
            sys.exit(0)
            
    except Exception as e:
        consecutive_errors += 1
        print(f"[{version}] Error checking block: {e}")
        if consecutive_errors >= 5:
            print(f"[{version}] Too many consecutive errors. Failing benchmark.")
            sys.exit(1)
    
    time.sleep(check_interval)