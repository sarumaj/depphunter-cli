#import <UIKit/UIKit.h>

NS_ASSUME_NONNULL_BEGIN

@class Cart;

@interface AppDelegate : UIResponder <UIApplicationDelegate>

@property (strong, nonatomic, nullable) UIWindow *window;
@property (nonatomic, copy) void (^onLaunch)(BOOL cold);

+ (instancetype)shared;
- (void)openCart:(Cart *)cart animated:(BOOL)animated NS_SWIFT_NAME(open(_:animated:));

@end

NS_ASSUME_NONNULL_END
