#import "Cart.h"

NSString * const CartDidChangeNotification = @"CartDidChange";

CartTotal CartTotalMake(double amount) {
    CartTotal t = { amount };
    return t;
}

static inline BOOL CartIsEmpty(NSArray *items) {
    return items.count == 0;
}

@implementation Cart

- (instancetype)initWithItems:(NSArray *)items {
    if ((self = [super init])) {
        _items = [items mutableCopy];
    }
    return self;
}

- (void)addItem:(id)item :(NSInteger)count {
    for (NSInteger i = 0; i < count; i++) { [_items addObject:item]; }
}

+ (NSString *)formatted:(NSString *)format, ... {
    return format;
}

- (NSArray *)items {
    return [_items copy];
}

@end
