#ifndef WRAPPER_H__
#define WRAPPER_H__
typedef unsigned int __u32;

typedef __u32 u32;

typedef signed char __s8;

typedef unsigned char __u8;

typedef short unsigned int __u16;

typedef int __s32;

typedef long long int __s64;

typedef long long unsigned int __u64;

typedef __s8 s8;

typedef __u8 u8;

typedef __u16 u16;

typedef __s32 s32;

typedef __s64 s64;

typedef __u64 u64;

enum uei_sizes {
	UEI_REASON_LEN		= 128,
	UEI_MSG_LEN		= 1024,
	UEI_DUMP_DFL_LEN	= 32768,
};

struct user_exit_info {
	int		kind;
	s64		exit_code;
	char	reason[UEI_REASON_LEN];
	char	msg[UEI_MSG_LEN];
};
#include "main.skeleton.h"

void *open_skel();

u32 get_usersched_pid();

void set_usersched_pid(u32 id);

void set_kugepagepid(u32 id);

void set_debug(bool enabled);

u64 get_nr_scheduled();

u64 get_nr_queued();

void notify_complete(u64 nr_pending);

void sub_nr_queued();

void destroy_skel(void *);

#endif