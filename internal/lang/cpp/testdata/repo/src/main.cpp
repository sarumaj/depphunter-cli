#include "local.h"
#include "app/app.hpp"
#include <fmt/core.h>
#include "net/socket.h"
#  include   <map>  // spaced out
#include <vector>
#include <cstdint>
#include <stdio.h>
#include <sys/socket.h>
#include <pthread.h>
#include <windows.h>
#include <boost/asio.hpp>
#include <openssl/ssl.h>
#include <zlib.h>
#include "config.h"
#include "gtest/gtest.h"
#include CONFIG_HEADER
/*
#include "commented.h"
*/
#if 0
#include <dead.h>
int dead_fn() { return 0; }
#define DEAD 1
#endif
#if 1
#define ALIVE 1
#else
#include <never.h>
#endif

namespace app {
Server::Server() {}
Server::~Server() {}
void Server::start() {}
static int helper(int x);
static int helper(int x) { return x; }
} // namespace app

struct Point {
  int x;
};
typedef void (*callback_t)(int);

int main() {
  struct Local {
    int z;
  };
  std::string s(nullptr);
  return 0;
}

TEST(Suite, Case) {}
