#ifndef UTIL_H
#define UTIL_H
typedef struct util_list {
  struct util_list *next;
} util_list_t;
int util_count(const char *s);
#endif
