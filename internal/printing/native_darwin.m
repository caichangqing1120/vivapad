#import <AppKit/AppKit.h>
#include <cups/cups.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

typedef struct vivapad_print_context {
    NSPrintInfo *info;
    NSFont *font;
} vivapad_print_context;

typedef struct {
    double paper_width, paper_height;
    double margin_left, margin_top, margin_right, margin_bottom;
    double header_line_height, footer_line_height, body_line_height;
    int selection_only, first_page, last_page, copies;
} vivapad_print_metrics;

static double vivapad_font_height(NSFont *font) {
    return ceil(font.ascender - font.descender + font.leading);
}

static NSFont *vivapad_band_font(int size, BOOL bold, BOOL italic) {
    NSFont *font = [NSFont fontWithName:@"Arial" size:size];
    if (font == nil) font = [NSFont systemFontOfSize:size];
    NSFontManager *manager = [NSFontManager sharedFontManager];
    if (bold) font = [manager convertFont:font toHaveTrait:NSBoldFontMask] ?: font;
    if (italic) font = [manager convertFont:font toHaveTrait:NSItalicFontMask] ?: font;
    return font;
}

int vivapad_print_choose(int interactive, int has_selection, double font_size,
    int header_size, int header_bold, int header_italic,
    int footer_size, int footer_bold, int footer_italic,
    vivapad_print_context **context, vivapad_print_metrics *metrics, char **message) {
    if (context == NULL || metrics == NULL || message == NULL) {
        return -1;
    }
    *context = NULL;
    *message = NULL;
    @autoreleasepool {
        if (interactive && ![NSThread isMainThread]) {
            *message = strdup("print dialog must run on the application thread");
            return -1;
        }
        NSPrintInfo *info = [[NSPrintInfo sharedPrintInfo] copy];
        if (info == nil) {
            *message = strdup("system print settings are unavailable");
            return -1;
        }
        if (!interactive) {
            const char *defaultName = cupsGetDefault();
            NSPrinter *defaultPrinter = defaultName != NULL ? [NSPrinter printerWithName:[NSString stringWithUTF8String:defaultName]] : nil;
            if (defaultPrinter == nil) {
                [info release];
                return 2;
            }
            info.printer = defaultPrinter;
        }
        info.selectionOnly = interactive && has_selection;
        if (interactive) {
            NSPrintPanel *panel = [NSPrintPanel printPanel];
            NSPrintPanelOptions options = panel.options | NSPrintPanelShowsCopies | NSPrintPanelShowsPageRange;
            if (has_selection) {
                options |= NSPrintPanelShowsPrintSelection;
            }
            panel.options = options;
            if ([panel runModalWithPrintInfo:info] != NSModalResponseOK) {
                [info release];
                return 1;
            }
        }
        if (info.printer == nil) {
            [info release];
            return 2;
        }
        NSFont *font = [NSFont userFixedPitchFontOfSize:font_size];
        NSFont *headerFont = vivapad_band_font(header_size, header_bold != 0, header_italic != 0);
        NSFont *footerFont = vivapad_band_font(footer_size, footer_bold != 0, footer_italic != 0);
        if (font == nil || headerFont == nil || footerFont == nil) {
            [info release];
            *message = strdup("native print fonts are unavailable");
            return -1;
        }
        NSSize paper = info.paperSize;
        NSRect printable = info.imageablePageBounds;
        metrics->paper_width = paper.width;
        metrics->paper_height = paper.height;
        metrics->margin_left = NSMinX(printable);
        metrics->margin_bottom = NSMinY(printable);
        metrics->margin_right = paper.width - NSMaxX(printable);
        metrics->margin_top = paper.height - NSMaxY(printable);
        metrics->header_line_height = vivapad_font_height(headerFont);
        metrics->footer_line_height = vivapad_font_height(footerFont);
        metrics->body_line_height = vivapad_font_height(font);
        NSDictionary *options = info.dictionary;
        metrics->selection_only = info.selectionOnly ? 1 : 0;
        metrics->copies = MAX(1, [options[NSPrintCopies] intValue]);
        if (options[NSPrintAllPages] == nil || [options[NSPrintAllPages] boolValue]) {
            metrics->first_page = 1;
            metrics->last_page = 65535;
        } else {
            metrics->first_page = MAX(1, [options[NSPrintFirstPage] intValue]);
            metrics->last_page = MAX(metrics->first_page, [options[NSPrintLastPage] intValue]);
        }
        vivapad_print_context *result = calloc(1, sizeof(*result));
        if (result == NULL) {
            [info release];
            *message = strdup("failed to allocate print context");
            return -1;
        }
        result->info = info;
        result->font = [font retain];
        *context = result;
        return 0;
    }
}

double vivapad_print_advance(vivapad_print_context *context, const char *utf8, size_t length) {
    if (context == NULL || context->font == nil || utf8 == NULL) {
        return NAN;
    }
    @autoreleasepool {
        NSString *value = [[[NSString alloc] initWithBytes:utf8 length:length encoding:NSUTF8StringEncoding] autorelease];
        if (value == nil) {
            return NAN;
        }
        return [value sizeWithAttributes:@{NSFontAttributeName: context->font}].width;
    }
}

@interface VivaPadPrintView : NSView {
    NSArray<NSDictionary *> *_pages;
    NSFont *_font;
    NSSize _paper;
    double _lineHeight;
}
- (instancetype)initWithPages:(NSArray<NSDictionary *> *)pages
                        paper:(NSSize)paper
                         font:(NSFont *)font
                   lineHeight:(double)lineHeight;
@end

@implementation VivaPadPrintView
- (instancetype)initWithPages:(NSArray<NSDictionary *> *)pages
                        paper:(NSSize)paper
                         font:(NSFont *)font
                   lineHeight:(double)lineHeight {
    self = [super initWithFrame:NSMakeRect(0, 0, paper.width, paper.height * pages.count)];
    if (self != nil) {
        _pages = [pages retain];
        _font = [font retain];
        _paper = paper;
        _lineHeight = lineHeight;
    }
    return self;
}

- (void)dealloc {
    [_pages release];
    [_font release];
    [super dealloc];
}

- (BOOL)isFlipped { return YES; }

- (BOOL)knowsPageRange:(NSRangePointer)range {
    *range = NSMakeRange(1, _pages.count);
    return YES;
}

- (NSRect)rectForPage:(NSInteger)page {
    return NSMakeRect(0, (page - 1) * _paper.height, _paper.width, _paper.height);
}

- (void)drawBand:(NSDictionary *)band
          bounds:(NSDictionary *)bounds
           atTop:(BOOL)atTop
          origin:(CGFloat)origin {
    NSString *left = band[@"left"] ?: @"";
    NSString *middle = band[@"middle"] ?: @"";
    NSString *right = band[@"right"] ?: @"";
    if (left.length == 0 && middle.length == 0 && right.length == 0) {
        return;
    }
    NSFont *font = vivapad_band_font([band[@"size"] intValue], [band[@"bold"] boolValue], [band[@"italic"] boolValue]);
    NSDictionary *attributes = @{NSFontAttributeName: font, NSForegroundColorAttributeName: [NSColor blackColor]};
    CGFloat lineHeight = vivapad_font_height(font);
    CGFloat leftEdge = [bounds[@"left"] doubleValue];
    CGFloat rightEdge = [bounds[@"right"] doubleValue];
    CGFloat baseline = origin + (atTop ? [bounds[@"top"] doubleValue] - 1.5 * lineHeight
                                     : [bounds[@"bottom"] doubleValue] + 0.5 * lineHeight);
    [left drawAtPoint:NSMakePoint(leftEdge, baseline) withAttributes:attributes];
    [middle drawAtPoint:NSMakePoint((leftEdge + rightEdge - [middle sizeWithAttributes:attributes].width) / 2, baseline)
        withAttributes:attributes];
    [right drawAtPoint:NSMakePoint(rightEdge - [right sizeWithAttributes:attributes].width, baseline)
        withAttributes:attributes];
    [[NSColor blackColor] setStroke];
    NSBezierPath *rule = [NSBezierPath bezierPath];
    rule.lineWidth = 0.5;
    CGFloat ruleY = atTop ? baseline + lineHeight + 2 : baseline - 3;
    [rule moveToPoint:NSMakePoint(leftEdge, ruleY)];
    [rule lineToPoint:NSMakePoint(rightEdge, ruleY)];
    [rule stroke];
}

- (void)drawRect:(NSRect)dirtyRect {
    NSInteger pageNumber = [NSPrintOperation currentOperation].currentPage;
    if (pageNumber < 1 || pageNumber > (NSInteger)_pages.count) {
        pageNumber = (NSInteger)floor(NSMinY(dirtyRect) / _paper.height) + 1;
    }
    if (pageNumber < 1 || pageNumber > (NSInteger)_pages.count) {
        return;
    }
    NSDictionary *page = _pages[pageNumber - 1];
    NSDictionary *bounds = page[@"bounds"];
    NSDictionary *textBounds = page[@"textBounds"];
    CGFloat origin = (pageNumber - 1) * _paper.height;
    [NSGraphicsContext saveGraphicsState];
    NSRectClip([self rectForPage:pageNumber]);
    [self drawBand:page[@"header"] bounds:bounds atTop:YES origin:origin];
    [self drawBand:page[@"footer"] bounds:bounds atTop:NO origin:origin];
    NSDictionary *numberStyle = @{NSFontAttributeName: _font,
        NSForegroundColorAttributeName: [NSColor colorWithSRGBRed:0.45 green:0.45 blue:0.45 alpha:1]};
    NSArray<NSDictionary *> *rows = page[@"rows"];
    for (NSUInteger index = 0; index < rows.count; index++) {
        NSDictionary *row = rows[index];
        CGFloat y = origin + [bounds[@"top"] doubleValue] + index * _lineHeight;
        if ([page[@"invertBody"] boolValue]) {
            [[NSColor blackColor] setFill];
            NSRectFill(NSMakeRect([textBounds[@"left"] doubleValue], y,
                [textBounds[@"right"] doubleValue] - [textBounds[@"left"] doubleValue], _lineHeight));
        }
        NSInteger number = [row[@"lineNumber"] integerValue];
        if (number > 0) {
            [[NSString stringWithFormat:@"%ld", (long)number]
                drawAtPoint:NSMakePoint([bounds[@"left"] doubleValue], y) withAttributes:numberStyle];
        }
        NSMutableAttributedString *line = [[[NSMutableAttributedString alloc] init] autorelease];
        for (NSDictionary *segment in row[@"segments"]) {
            NSColor *color = [NSColor colorWithSRGBRed:[segment[@"red"] doubleValue]
                                               green:[segment[@"green"] doubleValue]
                                                blue:[segment[@"blue"] doubleValue] alpha:1];
            NSDictionary *attributes = @{NSFontAttributeName: _font, NSForegroundColorAttributeName: color};
            NSAttributedString *part = [[NSAttributedString alloc] initWithString:segment[@"text"] attributes:attributes];
            [line appendAttributedString:part];
            [part release];
        }
        [line drawAtPoint:NSMakePoint([textBounds[@"left"] doubleValue], y)];
    }
    [NSGraphicsContext restoreGraphicsState];
}
@end

static VivaPadPrintView *vivapad_print_view(vivapad_print_context *context,
    const char *json, size_t length, char **message) {
    NSData *input = [NSData dataWithBytes:json length:length];
    NSError *parseError = nil;
    NSDictionary *document = [NSJSONSerialization JSONObjectWithData:input options:0 error:&parseError];
    NSArray<NSDictionary *> *pages = [document isKindOfClass:[NSDictionary class]] ? document[@"pages"] : nil;
    if (![pages isKindOfClass:[NSArray class]] || pages.count == 0) {
        *message = strdup(parseError.localizedDescription.UTF8String ?: "print pages are invalid");
        return nil;
    }
    NSSize paper = NSMakeSize([document[@"paperWidth"] doubleValue], [document[@"paperHeight"] doubleValue]);
    double lineHeight = [document[@"lineHeight"] doubleValue];
    if (!isfinite(paper.width) || !isfinite(paper.height) || paper.width <= 0 || paper.height <= 0 ||
        !isfinite(lineHeight) || lineHeight <= 0) {
        *message = strdup("print page dimensions are invalid");
        return nil;
    }
    return [[VivaPadPrintView alloc] initWithPages:pages paper:paper font:context->font lineHeight:lineHeight];
}

int vivapad_print_preview(vivapad_print_context *context, const char *json, size_t length,
    unsigned char **pdf, size_t *pdf_length, char **message) {
    if (context == NULL || json == NULL || length == 0 || pdf == NULL || pdf_length == NULL || message == NULL) {
        return -1;
    }
    *pdf = NULL;
    *pdf_length = 0;
    *message = NULL;
    @autoreleasepool {
        VivaPadPrintView *view = vivapad_print_view(context, json, length, message);
        if (view == nil) {
            return -1;
        }
        NSMutableData *output = [NSMutableData data];
        NSPrintOperation *operation = [NSPrintOperation PDFOperationWithView:view
            insideRect:view.bounds toData:output printInfo:context->info];
        operation.showsPrintPanel = NO;
        operation.showsProgressPanel = NO;
        BOOL rendered = [operation runOperation];
        [view release];
        if (!rendered || output.length == 0) {
            *message = strdup("native PDF print operation failed");
            return -1;
        }
        *pdf = malloc(output.length);
        if (*pdf == NULL) {
            *message = strdup("failed to allocate PDF preview");
            return -1;
        }
        memcpy(*pdf, output.bytes, output.length);
        *pdf_length = output.length;
        return 0;
    }
}

int vivapad_print_submit(vivapad_print_context *context, const char *json, size_t length,
    const char *save_path, char **message) {
    if (context == NULL || json == NULL || length == 0 || message == NULL) {
        return -1;
    }
    *message = NULL;
    @autoreleasepool {
        VivaPadPrintView *view = vivapad_print_view(context, json, length, message);
        if (view == nil) {
            return -1;
        }
        NSMutableDictionary *settings = context->info.dictionary;
        id oldDisposition = [settings[NSPrintJobDisposition] retain];
        id oldURL = [settings[NSPrintJobSavingURL] retain];
        NSString *destination = save_path != NULL ? [NSString stringWithUTF8String:save_path] : nil;
        if (save_path != NULL && destination == nil) {
            [view release];
            [oldDisposition release];
            [oldURL release];
            *message = strdup("PDF destination is not valid UTF-8");
            return -1;
        }
        if (destination != nil) {
            settings[NSPrintJobDisposition] = NSPrintSaveJob;
            settings[NSPrintJobSavingURL] = [NSURL fileURLWithPath:destination];
        }
        NSPrintOperation *operation = [NSPrintOperation printOperationWithView:view printInfo:context->info];
        operation.showsPrintPanel = NO;
        operation.showsProgressPanel = NO;
        BOOL submitted = [operation runOperation];
        if (destination != nil) {
            if (oldDisposition != nil) settings[NSPrintJobDisposition] = oldDisposition;
            else [settings removeObjectForKey:NSPrintJobDisposition];
            if (oldURL != nil) settings[NSPrintJobSavingURL] = oldURL;
            else [settings removeObjectForKey:NSPrintJobSavingURL];
        }
        [oldDisposition release];
        [oldURL release];
        [view release];
        if (!submitted) {
            *message = strdup("native print operation failed");
            return -1;
        }
        if (destination != nil) {
            NSDictionary *file = [[NSFileManager defaultManager] attributesOfItemAtPath:destination error:nil];
            if ([file fileSize] == 0) {
                *message = strdup("native print operation did not write the PDF destination");
                return -1;
            }
        }
        return 0;
    }
}

void vivapad_print_close(vivapad_print_context *context) {
    if (context == NULL) {
        return;
    }
    [context->font release];
    [context->info release];
    free(context);
}
