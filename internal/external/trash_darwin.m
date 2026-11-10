#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>

int vivapad_trash_file(const char *path, char **location, char **message) {
    @autoreleasepool {
        NSString *source = [NSString stringWithUTF8String:path];
        if (source == nil) {
            *message = strdup("file path is not valid UTF-8");
            return -1;
        }
        NSError *error = nil;
        NSURL *destination = nil;
        BOOL moved = [[NSFileManager defaultManager]
            trashItemAtURL:[NSURL fileURLWithPath:source]
            resultingItemURL:&destination error:&error];
        if (!moved) {
            *message = strdup(error.localizedDescription.UTF8String ?: "Trash operation failed");
            return -1;
        }
        if (location != NULL) {
            *location = strdup(destination.path.UTF8String ?: "");
        }
        return 0;
    }
}
