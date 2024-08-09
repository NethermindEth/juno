use std::{fs, process::Command};

use url::Url;

use toml::{self, Table};

fn main() {
    let cairo_native = "cairo-native";

    println!("cargo::rerun-if-changed=./Cargo.toml");
    println!("cargo::rerun-if-changed=./Cargo.lock");
    let config = fs::read_to_string("./Cargo.toml").unwrap();
    let t: Table = toml::from_str(&config).unwrap();
    let cairo_native_toml_table = t
        .get("dependencies")
        .expect("No dependencies found")
        .get(cairo_native)
        .expect("dependency cairo_native not found");

    // Dependency specified via path?
    let maybe_path = cairo_native_toml_table
        .get("path")
        .and_then(|path| path.as_str());

    let lock_file = fs::read_to_string("./Cargo.lock").unwrap();
    let lock_table: Table = toml::from_str(&lock_file).unwrap();
    let package = lock_table.get("package").unwrap().as_array().unwrap();
    let cairo_native_lock_table = package
        .iter()
        .find(|v| v.get("name").unwrap().as_str().unwrap() == cairo_native)
        .unwrap()
        .as_table()
        .unwrap();

    // Dependency specified via registry?
    let maybe_checksum = cairo_native_lock_table
        .get("checksum")
        .and_then(|c| c.as_str());

    // Dependency specified via git?
    let maybe_rev = extract_rev_from_source(cairo_native_lock_table);

    let native_version: String = match (maybe_rev, maybe_path, maybe_checksum) {
        (Some(rev), _, _) => rev,
        (_, Some(path), _) => get_git_rev(path),
        (_, _, Some(checksum)) => checksum.to_string(),
        (_, _, _) => panic!("No revision, path, or checksum found"),
    };

    // Set environment variable NATIVE_CACHE_VERSION
    println!("cargo::rustc-env=NATIVE_VERSION={native_version}");
}

fn get_git_rev(path: &str) -> String {
    let path = fs::canonicalize(path).unwrap();
    let out = Command::new("git")
        .args(["rev-parse", "HEAD"])
        .current_dir(path)
        .output();
    let git_rev = String::from_utf8(out.unwrap().stdout);
    git_rev.unwrap()
}

/// Extract the revision from git source
fn extract_rev_from_source(lock_table: &toml::Table) -> Option<String> {
    let source = Url::parse(lock_table.get("source")?.as_str()?).ok()?;
    Some(String::from(
        source
            .fragment()
            .expect("source must have a revision fragment"),
    ))
}
