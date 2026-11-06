//go:build android && cgo

package client

/*
#cgo LDFLAGS: -ldl -llog

#include <jni.h>
#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <android/log.h>
#include <sys/system_properties.h>

typedef jint (*get_vms_fn)(JavaVM **, jsize, jsize *);

static void alog_msg(const char *msg) {
	__android_log_print(ANDROID_LOG_INFO, "frp", "%s", msg != NULL ? msg : "(null)");
}

static void alog_kv(const char *key, const char *val) {
	__android_log_print(ANDROID_LOG_INFO, "frp", "%s=%s", key, val != NULL ? val : "(null)");
}

static void clear_exc(JNIEnv *env) {
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionClear(env);
	}
}

static char *dup_jstring(JNIEnv *env, jstring s) {
	const char *utf;
	char *out;
	if (s == NULL) {
		return NULL;
	}
	utf = (*env)->GetStringUTFChars(env, s, NULL);
	if (utf == NULL) {
		return NULL;
	}
	out = strdup(utf);
	(*env)->ReleaseStringUTFChars(env, s, utf);
	return out;
}

static int usable_name(const char *s) {
	return s != NULL && s[0] != '\0' && strcmp(s, "localhost") != 0;
}

static char *read_prop(const char *key) {
	char buf[PROP_VALUE_MAX];
	int n = __system_property_get(key, buf);
	if (n <= 0) {
		alog_kv(key, NULL);
		return NULL;
	}
	alog_kv(key, buf);
	if (!usable_name(buf)) {
		return NULL;
	}
	return strdup(buf);
}

static char *first_prop(void) {
	static const char *keys[] = {
		"persist.sys.device_name",
		"ro.product.marketname",
		"ro.product.odm.marketname",
		"ro.product.vendor.marketname",
		"ro.product.model",
	};
	size_t i;
	for (i = 0; i < sizeof keys / sizeof keys[0]; i++) {
		char *val = read_prop(keys[i]);
		if (val != NULL) {
			return val;
		}
	}
	return NULL;
}

static int copy_prop(const char *key, char *buf, size_t buflen) {
	char tmp[PROP_VALUE_MAX];
	int n = __system_property_get(key, tmp);
	if (buflen > 0) {
		buf[0] = '\0';
	}
	if (n <= 0 || !usable_name(tmp)) {
		alog_kv(key, n > 0 ? tmp : NULL);
		return 0;
	}
	alog_kv(key, tmp);
	snprintf(buf, buflen, "%s", tmp);
	return 1;
}

static int starts_with_ci(const char *s, const char *prefix) {
	while (*prefix != '\0') {
		char a = *s;
		char b = *prefix;
		if (a >= 'A' && a <= 'Z') {
			a = (char)(a - 'A' + 'a');
		}
		if (b >= 'A' && b <= 'Z') {
			b = (char)(b - 'A' + 'a');
		}
		if (a == '\0' || a != b) {
			return 0;
		}
		s++;
		prefix++;
	}
	return 1;
}

static void cap_first(char *s) {
	if (s[0] >= 'a' && s[0] <= 'z') {
		s[0] = (char)(s[0] - 'a' + 'A');
	}
}

static char *with_model(char *name) {
	char brand[PROP_VALUE_MAX];
	char model[PROP_VALUE_MAX];
	char label[PROP_VALUE_MAX * 2 + 2];
	char *out;
	size_t len;

	if (!usable_name(name)) {
		return name;
	}
	if (!copy_prop("ro.product.manufacturer", brand, sizeof brand) &&
		!copy_prop("ro.product.brand", brand, sizeof brand)) {
		brand[0] = '\0';
	}
	cap_first(brand);
	if (!copy_prop("ro.product.model", model, sizeof model)) {
		return name;
	}
	if (brand[0] != '\0' && !starts_with_ci(model, brand)) {
		snprintf(label, sizeof label, "%s %s", brand, model);
	} else {
		snprintf(label, sizeof label, "%s", model);
	}
	alog_kv("hostname: model label", label);
	if (strcmp(name, label) == 0) {
		return name;
	}
	len = strlen(name) + strlen(label) + 4;
	out = (char *)malloc(len);
	if (out == NULL) {
		return name;
	}
	snprintf(out, len, "%s (%s)", name, label);
	free(name);
	return out;
}

static char *finish(char *result) {
	if (!usable_name(result)) {
		free(result);
		return NULL;
	}
	result = with_model(result);
	alog_kv("hostname: chosen", result);
	return result;
}

static JavaVM *vm_from_art(void) {
	FILE *f;
	char line[1024];
	char *path;
	void *handle;
	get_vms_fn getvms;
	JavaVM *vm = NULL;
	jsize n = 0;

	f = fopen("/proc/self/maps", "r");
	if (f == NULL) {
		alog_msg("hostname: cannot open /proc/self/maps");
		return NULL;
	}
	while (fgets(line, sizeof line, f) != NULL) {
		path = strchr(line, '/');
		if (path == NULL || strstr(path, "libart.so") == NULL) {
			continue;
		}
		path[strcspn(path, " \n")] = '\0';
		alog_kv("hostname: libart", path);
		handle = dlopen(path, RTLD_NOW);
		if (handle == NULL) {
			alog_kv("hostname: dlopen libart", dlerror());
			break;
		}
		getvms = (get_vms_fn)dlsym(handle, "JNI_GetCreatedJavaVMs");
		if (getvms == NULL) {
			alog_msg("hostname: libart has no JNI_GetCreatedJavaVMs");
			dlclose(handle);
			break;
		}
		if (getvms(&vm, 1, &n) != JNI_OK || n < 1 || vm == NULL) {
			alog_msg("hostname: libart JNI_GetCreatedJavaVMs returned no VM");
			vm = NULL;
		}
		dlclose(handle);
		break;
	}
	fclose(f);
	return vm;
}

static char *android_device_name(void) {
	JavaVM *vm = NULL;
	JNIEnv *env = NULL;
	jsize n = 0;
	int attached = 0;
	jint st;
	char *result = NULL;

	jclass at;
	jmethodID cur;
	jobject app;
	jclass ctx;
	jmethodID gcr;
	jobject resolver;
	jclass settings;
	jmethodID getString;
	jstring key;
	jstring val;

	jclass build;
	jfieldID modelID;
	jstring model;
	get_vms_fn getvms;

	getvms = (get_vms_fn)dlsym(RTLD_DEFAULT, "JNI_GetCreatedJavaVMs");
	if (getvms == NULL) {
		alog_msg("hostname: JNI_GetCreatedJavaVMs symbol not found");
		vm = vm_from_art();
	} else if (getvms(&vm, 1, &n) != JNI_OK || n < 1 || vm == NULL) {
		alog_msg("hostname: JNI_GetCreatedJavaVMs returned no VM");
		vm = NULL;
	}
	if (vm == NULL) {
		return finish(first_prop());
	}

	st = (*vm)->GetEnv(vm, (void **)&env, JNI_VERSION_1_6);
	if (st == JNI_EDETACHED) {
		if ((*vm)->AttachCurrentThread(vm, &env, NULL) != JNI_OK) {
			alog_msg("hostname: AttachCurrentThread failed");
			return finish(first_prop());
		}
		attached = 1;
	} else if (st != JNI_OK) {
		alog_msg("hostname: GetEnv failed");
		return finish(first_prop());
	}

	at = (*env)->FindClass(env, "android/app/ActivityThread");
	clear_exc(env);
	if (at == NULL) {
		alog_msg("hostname: ActivityThread class not found");
	}
	if (at != NULL) {
		cur = (*env)->GetStaticMethodID(env, at, "currentApplication", "()Landroid/app/Application;");
		clear_exc(env);
		if (cur != NULL) {
			app = (*env)->CallStaticObjectMethod(env, at, cur);
			clear_exc(env);
			if (app == NULL) {
				alog_msg("hostname: currentApplication is null");
			}
			if (app != NULL) {
				ctx = (*env)->GetObjectClass(env, app);
				gcr = (*env)->GetMethodID(env, ctx, "getContentResolver", "()Landroid/content/ContentResolver;");
				clear_exc(env);
				if (gcr != NULL) {
					resolver = (*env)->CallObjectMethod(env, app, gcr);
					clear_exc(env);
					if (resolver != NULL) {
						settings = (*env)->FindClass(env, "android/provider/Settings$Global");
						clear_exc(env);
						if (settings != NULL) {
							getString = (*env)->GetStaticMethodID(
								env, settings, "getString",
								"(Landroid/content/ContentResolver;Ljava/lang/String;)Ljava/lang/String;");
							clear_exc(env);
							if (getString != NULL) {
								key = (*env)->NewStringUTF(env, "device_name");
								val = (jstring)(*env)->CallStaticObjectMethod(env, settings, getString, resolver, key);
								clear_exc(env);
								result = dup_jstring(env, val);
								alog_kv("hostname: Settings.Global.device_name", result);
							}
						}
					}
				}
			}
		}
	}
	if (result == NULL) {
		alog_msg("hostname: Settings.Global.device_name was not read");
	}

	if (!usable_name(result)) {
		free(result);
		result = NULL;
		build = (*env)->FindClass(env, "android/os/Build");
		clear_exc(env);
		if (build != NULL) {
			modelID = (*env)->GetStaticFieldID(env, build, "MODEL", "Ljava/lang/String;");
			clear_exc(env);
			if (modelID != NULL) {
				model = (jstring)(*env)->GetStaticObjectField(env, build, modelID);
				clear_exc(env);
				result = dup_jstring(env, model);
				alog_kv("hostname: Build.MODEL", result);
			}
		}
	}
	if (result == NULL) {
		alog_msg("hostname: Build.MODEL was not read");
	}

	if (attached) {
		(*vm)->DetachCurrentThread(vm);
	}
	if (!usable_name(result)) {
		free(result);
		result = first_prop();
	}
	return finish(result);
}
*/
import "C"
import (
	"fmt"
	"os"
	"strings"
	"unsafe"
)

func androidLog(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	cstr := C.CString(msg)
	C.alog_msg(cstr)
	C.free(unsafe.Pointer(cstr))
}

func machineHostname() string {
	if p := C.android_device_name(); p != nil {
		name := strings.TrimSpace(C.GoString(p))
		C.free(unsafe.Pointer(p))
		androidLog("hostname: candidate %q", name)
		if name != "" && !strings.EqualFold(name, "localhost") {
			androidLog("hostname: use %q", name)
			return name
		}
	} else {
		androidLog("hostname: candidate null")
	}
	hostname, err := os.Hostname()
	androidLog("hostname: os.Hostname=%q err=%v", hostname, err)
	hostname = strings.TrimSpace(hostname)
	if err == nil && hostname != "" && !strings.EqualFold(hostname, "localhost") {
		androidLog("hostname: use os.Hostname %q", hostname)
		return hostname
	}
	androidLog("hostname: use fallback %q", "")
	return ""
}
