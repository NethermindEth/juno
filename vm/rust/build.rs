fn main() {
    let mut config = prost_build::Config::new();
    config.out_dir("src/protobuf");
    config
        .compile_protos(&["../protobuf/compiled_class.proto"], &["../protobuf"])
        .unwrap();
    println!("cargo:rerun-if-changed=../protobuf/compiled_class.proto");
}