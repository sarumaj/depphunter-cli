#ifndef APP_APP_HPP
#define APP_APP_HPP

#include "detail.hpp"
#include <string>

#define APP_VERSION "1.0"
#define APP_MAX(a, b) ((a) > (b) ? (a) : (b))

namespace app {

class Server {
public:
  Server();
  ~Server();
  void start();
  int port() const { return port_; }
  bool operator==(const Server &other) const;
  struct Options {
    int threads;
  };

private:
  int port_ = 0;
};

enum class Mode { Fast, Safe };
union Value {
  int i;
  float f;
};
using Name = std::string;
typedef int Id;

template <typename T> T twice(T v) { return v + v; }
int parse(const char *s);

} // namespace app

#endif
