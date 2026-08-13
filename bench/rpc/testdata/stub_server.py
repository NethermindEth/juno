#!/usr/bin/env python3

import argparse
import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    ready_checks = 0
    ready_lock = threading.Lock()
    mode = "ok"
    ready_failures = 0
    juno_version = "sha-0123456789abcdef"

    def log_message(self, _format, *_args):
        return

    def send_json(self, status, payload):
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path != "/ready/rpc":
            self.send_json(404, {"error": "not found"})
            return

        with self.ready_lock:
            type(self).ready_checks += 1
            ready_check = type(self).ready_checks

        if self.mode == "not-ready" or ready_check <= self.ready_failures:
            self.send_json(503, {"ready": False})
            return
        self.send_json(200, {"ready": True})

    def do_POST(self):
        try:
            length = int(self.headers.get("Content-Length", "0"))
            request = json.loads(self.rfile.read(length))
        except (ValueError, json.JSONDecodeError):
            self.send_json(400, {"error": "invalid request"})
            return

        requests = request if isinstance(request, list) else [request]
        responses = [self.rpc_response(item) for item in requests]
        self.send_json(200, responses if isinstance(request, list) else responses[0])

    def rpc_response(self, request):
        method = request.get("method")
        response = {"jsonrpc": "2.0", "id": request.get("id")}
        if method == "juno_version":
            response["result"] = self.juno_version
        elif method == "starknet_chainId":
            response["result"] = "0x534e5f4d41494e"
        elif method == "starknet_blockNumber":
            response["result"] = 800000
        elif method == "starknet_getTransactionByHash" and self.mode == "rpc-error":
            response["error"] = {"code": 25, "message": "Transaction hash not found"}
        elif method == "starknet_getTransactionByHash":
            response["result"] = {
                "transaction_hash": request["params"]["transaction_hash"],
                "type": "INVOKE",
            }
        else:
            response["error"] = {"code": -32601, "message": "Method not found"}
        return response


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=("ok", "not-ready", "rpc-error"), default="ok")
    parser.add_argument("--ready-failures", type=int, default=0)
    parser.add_argument("--juno-version", default="sha-0123456789abcdef")
    parser.add_argument("--port-file", required=True)
    args = parser.parse_args()

    Handler.mode = args.mode
    Handler.ready_failures = args.ready_failures
    Handler.juno_version = args.juno_version
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    with open(args.port_file, "w", encoding="utf-8") as port_file:
        port_file.write(str(server.server_port))
    server.serve_forever()


if __name__ == "__main__":
    main()
