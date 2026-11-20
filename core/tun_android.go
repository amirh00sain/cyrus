//go:build android

package core

/*
#include <jni.h>
#include <stdint.h>

// Loads an app class through the Activity's class loader. FindClass() would
// only see system classes when called from a native thread.
static jclass operaFindClass(JNIEnv* env, jobject ctx, const char* name) {
	jclass ctxCls = (*env)->GetObjectClass(env, ctx);
	jmethodID getLoader = (*env)->GetMethodID(env, ctxCls, "getClassLoader", "()Ljava/lang/ClassLoader;");
	if (getLoader == NULL) { (*env)->ExceptionClear(env); return NULL; }
	jobject loader = (*env)->CallObjectMethod(env, ctx, getLoader);
	if (loader == NULL) { (*env)->ExceptionClear(env); return NULL; }
	jclass loaderCls = (*env)->GetObjectClass(env, loader);
	jmethodID loadClass = (*env)->GetMethodID(env, loaderCls, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
	if (loadClass == NULL) { (*env)->ExceptionClear(env); return NULL; }
	jstring jname = (*env)->NewStringUTF(env, name);
	jclass cls = (jclass)(*env)->CallObjectMethod(env, loader, loadClass, jname);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return NULL; }
	return cls;
}

// op: 0 request(connect)  1 request(permission only)  2 state  3 fd  4 stop  5 reset
// Returns -100 when the Java part is missing from the APK.
static int operaCall(uintptr_t envp, uintptr_t ctxp, int op) {
	JNIEnv* env = (JNIEnv*)envp;
	jobject ctx = (jobject)ctxp;
	jclass cls = operaFindClass(env, ctx, "com.opera.vpn.VpnBridge");
	if (cls == NULL) return -100;
	jmethodID m = NULL;
	int result = 0;
	switch (op) {
	case 0:
	case 1:
		m = (*env)->GetStaticMethodID(env, cls, "request", "(Landroid/content/Context;Z)V");
		if (m == NULL) { (*env)->ExceptionClear(env); return -100; }
		(*env)->CallStaticVoidMethod(env, cls, m, ctx, op == 0 ? JNI_TRUE : JNI_FALSE);
		break;
	case 2:
		m = (*env)->GetStaticMethodID(env, cls, "state", "()I");
		if (m == NULL) { (*env)->ExceptionClear(env); return -100; }
		result = (*env)->CallStaticIntMethod(env, cls, m);
		break;
	case 3:
		m = (*env)->GetStaticMethodID(env, cls, "fd", "()I");
		if (m == NULL) { (*env)->ExceptionClear(env); return -100; }
		result = (*env)->CallStaticIntMethod(env, cls, m);
		break;
	case 4:
		m = (*env)->GetStaticMethodID(env, cls, "stop", "(Landroid/content/Context;)V");
		if (m == NULL) { (*env)->ExceptionClear(env); return -100; }
		(*env)->CallStaticVoidMethod(env, cls, m, ctx);
		break;
	case 5:
		m = (*env)->GetStaticMethodID(env, cls, "reset", "()V");
		if (m == NULL) { (*env)->ExceptionClear(env); return -100; }
		(*env)->CallStaticVoidMethod(env, cls, m);
		break;
	}
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return -101; }
	return result;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2/driver"
)

// VpnBridge.state() values (see android/java/com/opera/vpn/VpnBridge.java).
const (
	vpnIdle    = 0
	vpnWaiting = 1 // consent dialog is showing
	vpnRunning = 2 // TUN interface established
	vpnDenied  = 3 // user refused the VPN permission
	vpnFailed  = 4
)

const (
	opRequestConnect = 0
	opRequestPermit  = 1
	opState          = 2
	opFd             = 3
	opStop           = 4
	opReset          = 5
)

var errNoJava = errors.New("the VPN service is missing from this APK build (see android/README.md); use Proxy mode")

// androidCall runs one bridge call on the Android UI thread with a valid JNI
// environment and Context.
func androidCall(op int) (int, error) {
	res := 0
	err := driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errors.New("no Android context available")
		}
		res = int(C.operaCall(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), C.int(op)))
		return nil
	})
	if err != nil {
		return 0, err
	}
	if res == -100 {
		return 0, errNoJava
	}
	if res == -101 {
		return 0, errors.New("VPN service call raised a Java exception")
	}
	return res, nil
}

// RequestVPNPermission shows Android's "Connection request" dialog without
// starting a VPN. Used by the "Grant VPN permission" button in the app.
func RequestVPNPermission() error {
	_, err := androidCall(opRequestPermit)
	return err
}

var tunMu sync.Mutex
var tunUp bool

func tunSupported() bool { return true }

func startTun(proxyAddress string, _ bool) error {
	tunMu.Lock()
	defer tunMu.Unlock()
	if tunUp {
		return nil
	}
	if _, err := androidCall(opReset); err != nil {
		return err
	}
	// Opens the transparent permission activity, which shows the system VPN
	// consent dialog if needed and then starts the VpnService.
	if _, err := androidCall(opRequestConnect); err != nil {
		return err
	}

	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		st, err := androidCall(opState)
		if err != nil {
			return err
		}
		switch st {
		case vpnRunning:
			fd, err := androidCall(opFd)
			if err != nil {
				return err
			}
			if fd < 0 {
				return errors.New("the VPN interface has no file descriptor")
			}
			host, port := splitAddress(proxyAddress)
			if err := startT2S(fmt.Sprintf("fd://%d", fd), "socks5://"+host+":"+port); err != nil {
				_, _ = androidCall(opStop)
				return err
			}
			tunUp = true
			return nil
		case vpnDenied:
			return errors.New("VPN permission was denied - allow it to use TUN mode")
		case vpnFailed:
			return errors.New("Android could not create the VPN interface")
		}
		time.Sleep(250 * time.Millisecond)
	}
	_, _ = androidCall(opStop)
	return errors.New("timed out while waiting for the VPN permission dialog")
}

func stopTun() error {
	tunMu.Lock()
	defer tunMu.Unlock()
	stopT2S()
	_, _ = androidCall(opStop)
	tunUp = false
	return nil
}

