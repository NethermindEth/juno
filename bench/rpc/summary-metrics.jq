def metric_value($name; $key):
  (.metrics[$name] // {}) as $metric
  | ($metric[$key] // $metric.values[$key] // 0)
  | if type == "number" then . else error("invalid metric value") end;

{
  failedChecks: metric_value("checks"; "fails"),
  requestFailures: metric_value("rpc_request_failures"; "count"),
  httpRequestFailures: metric_value("http_req_failed"; "passes"),
  vuFailures: metric_value("vu_failures"; "count"),
  droppedIterations: metric_value("dropped_iterations"; "count"),
  completedIterations: metric_value("iterations"; "count")
}
