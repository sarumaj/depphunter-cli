#import "AppDelegate.h"
#import "Models/Cart.h"
#import "Models/Cart+Pricing.h"
#import <AFNetworking/AFNetworking.h>
#import <SDWebImage/UIImageView+WebCache.h>
#import "Masonry.h"
#import "AFURLSessionManager.h"
#import <GRDB/GRDB.h>
#import <LocalKit/LKThing.h>
#import <objc/runtime.h>
#import <Appkit/Appkit.h>
#include <stdio.h>
#import <Unknown/Unknown.h>
#import "Generated.h"
@import Firebase;
@import CoreData.NSManagedObject;

#if 0
#import <Removed/Removed.h>
#endif

static NSString * const kLaunchKey = @"launch";

@interface AppDelegate ()
@property (nonatomic) NSInteger launches;
- (void)track;
@end

@implementation AppDelegate

+ (instancetype)shared {
    static AppDelegate *instance;
    static dispatch_once_t once;
    dispatch_once(&once, ^{ instance = [AppDelegate new]; });
    return instance;
}

- (void)openCart:(Cart *)cart animated:(BOOL)animated {
    if (animated) { [self track]; }
}

- (void)track {
    NSLog(@"}{ %@", kLaunchKey);
}

@end
