#!/bin/sh
set -eu
cd "${1:?usage: build.sh /path/to/skynet}"
# GCC 4.8 compatibility; upstream fallback omits atomic_pause_ outside C11.
# Use glibc malloc because GitHub source archives omit the jemalloc submodule.
make linux -j4 CC='gcc -D__STDC_NO_ATOMICS__ -D_Thread_local=__thread -Datomic_pause_=__builtin_ia32_pause' \
  MYCFLAGS="-std=gnu11 -I$(pwd)/skynet-src" MALLOC_STATICLIB= SKYNET_DEFINES=-DNOUSE_JEMALLOC
