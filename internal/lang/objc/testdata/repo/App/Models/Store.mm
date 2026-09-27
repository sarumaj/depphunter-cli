#import "Cart.h"
#include <vector>
#include "Store.hpp"

namespace shop {
class Ledger {
public:
    void add(double v) { values.push_back(v); }
private:
    std::vector<double> values;
};

double sum(const std::vector<double> &v) {
    double s = 0;
    for (auto x : v) s += x;
    return s;
}
}

@interface Store : NSObject
@end

@implementation Store
- (void)checkout {
    auto raw = R"(not a "string" end)";
    (void)raw;
}
@end
