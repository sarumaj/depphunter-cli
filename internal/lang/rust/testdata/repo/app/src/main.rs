use std::collections::HashMap;
use crate::net::{self, server::Server as Srv};
use serde::Deserialize;
use tokio::runtime;
use core_lib::util;
use json::Value;
use rand::Rng;
extern crate alloc;
mod net;
use Mode::*;
mod config;

pub fn main() {}

struct App<T>(T);

impl<T> App<T> {
    fn run(&self) {}
}
