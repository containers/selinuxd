#include <semanage.h>
#include <stdlib.h>
#include <stdarg.h>
#include <stdio.h>
#include <sepol/cil/cil.h>

#include "_cgo_export.h"

extern void LogWrapper(char *, int);

static void semanage_error_callback(void *varg, semanage_handle_t *handle, const char *fmt, ...)
{
    char log_msg[1024];
    va_list ap;

    va_start(ap, fmt);
    vsnprintf(log_msg, sizeof(log_msg)-1, fmt, ap);
	LogWrapper(log_msg, semanage_msg_get_level(handle));
    va_end(ap);
}

static void cil_log_callback(int level, char *message)
{
	LogWrapper(message, level);
}

void wrap_set_cb(semanage_handle_t *handle, void *arg)
{
	cil_set_log_handler(cil_log_callback);
	semanage_msg_set_callback(handle, semanage_error_callback, arg);
}
