use std::{fs, process::Command};

use url::Url;

use toml::{self, Table};

fn main() {
    let cairo_native = "cairo-native";

    // Can you have globs?
    println!("cargo::rerun-if-changed=./Cargo.toml");
    println!("cargo::rerun-if-changed=./Cargo.lock");
    let config = fs::read_to_string("./Cargo.toml").unwrap();
    let t: Table = toml::from_str(&config).unwrap();
    let b = t
        .get("dependencies")
        .expect("No dependencies found") // TODO(xrvdg) see TOML on how this section should be called
        .get(cairo_native)
        .expect("dependency {cairo_native} not found"); // TODO(xrvdg) how to create a format message?

    // Both of these are error that should not happen
    // because if this doesn't exist it also doesn't exist in the lock file

    // How do you chain the values? You can keep using and_then, but then you can't get give feedback over
    // file not found

    let maybe_path = b.get("path").and_then(|path| path.as_str());

    // todo(xrvdg) Read TigerBeetle naming convention
    let lock_file = fs::read_to_string("./Cargo.lock").unwrap();
    let lock_table: Table = toml::from_str(&lock_file).unwrap();
    let package = lock_table.get("package").unwrap().as_array().unwrap();
    let cairo_native_lock_table = package
        .iter()
        .find(|v| v.get("name").unwrap().as_str().unwrap() == cairo_native)
        .unwrap()
        .as_table()
        .unwrap();

    // checksum from if it comes from registry
    let maybe_checksum = cairo_native_lock_table
        .get("checksum")
        .and_then(|c| c.as_str());

    // Should work for both unspecified revision and specified revision, at least for github links
    // source is optional
    // Todo(xrvdg) could probably use ? if I make it into a function
    let maybe_rev = extract_rev_from_source(cairo_native_lock_table);

    let rn: String = match (maybe_rev, maybe_path, maybe_checksum) {
        (Some(rev), _, _) => rev,
        (_, Some(path), _) => get_git_rev(path),
        (_, _, Some(checksum)) => checksum.to_string(),
        (_, _, _) => panic!("No revision, path, or checksum found"),
    };

    // Set environment variable NATIVE_CACHE_VERSION
    println!("cargo::rustc-env=NATIVE_VERSION={rn}");
}

fn get_git_rev(path: &str) -> String {
    let path = fs::canonicalize(path).unwrap();
    let p = Command::new("git")
        .args(["rev-parse", "HEAD"])
        .current_dir(path)
        .output();
    // TODO(xrvdg) convert to expects
    let out = String::from_utf8(p.unwrap().stdout);
    out.unwrap()
}

fn extract_rev_from_source(lock_table: &toml::Table) -> Option<String> {
    // We now swallow a parse error. This makes it a bit harder to debug if something goes wrong
    // Can also add warning and use the panic that comes later
    let source = Url::parse(lock_table.get("source")?.as_str()?).ok()?;
    Some(String::from(
        source
            .fragment()
            .expect("source must have a revision fragment"),
    ))
}
