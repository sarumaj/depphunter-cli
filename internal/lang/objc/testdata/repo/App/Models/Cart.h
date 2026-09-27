#ifndef CART_H
#define CART_H

#import <Foundation/Foundation.h>

#define CART_MAX_ITEMS 99
#define CART_LOG(fmt, ...) NSLog(fmt, ##__VA_ARGS__)

typedef NS_ENUM(NSInteger, CartState) {
    CartStateOpen,
    CartStateClosed,
};

typedef NS_OPTIONS(NSUInteger, CartFlags) {
    CartFlagGift = 1 << 0,
};

typedef void (^CartCompletion)(BOOL ok, NSError * _Nullable error);

typedef struct {
    double amount;
} CartTotal;

FOUNDATION_EXPORT NSString * const CartDidChangeNotification;

extern CartTotal CartTotalMake(double amount);

@protocol CartObserver <NSObject>
- (void)cartDidChange:(id)cart;
@optional
- (void)cart:(id)cart didAdd:(NSString *)item;
@end

@interface Cart<ObjectType> : NSObject <NSCopying>
{
    NSMutableArray *_items;
}
@property (nonatomic, readonly) NSArray<ObjectType> *items;
@property (class, nonatomic, readonly) Cart *current;
- (instancetype)initWithItems:(NSArray<ObjectType> *)items NS_DESIGNATED_INITIALIZER;
- (void)addItem:(ObjectType)item :(NSInteger)count;
+ (NSString *)formatted:(NSString *)format, ... NS_FORMAT_FUNCTION(1, 2);
@end

#endif
