import syslog

# syslog is the system-logger wrapper. Its LOG_* values are the POSIX-standard
# numbers, identical on darwin and linux for the common set, so the fixture
# checks those values and the mask helpers; the actual log delivery goes to the
# system log, not stdout, so it is exercised for non-raising but not observed.

# Priorities are the fixed POSIX severities.
print("priorities:", syslog.LOG_EMERG, syslog.LOG_ALERT, syslog.LOG_CRIT,
      syslog.LOG_ERR, syslog.LOG_WARNING, syslog.LOG_NOTICE, syslog.LOG_INFO,
      syslog.LOG_DEBUG)

# Option flags and the common facilities carry the standard values on both hosts.
print("options:", syslog.LOG_PID, syslog.LOG_CONS, syslog.LOG_ODELAY, syslog.LOG_NDELAY)
print("facilities:", syslog.LOG_KERN, syslog.LOG_USER, syslog.LOG_MAIL,
      syslog.LOG_DAEMON, syslog.LOG_AUTH, syslog.LOG_LPR, syslog.LOG_NEWS,
      syslog.LOG_UUCP, syslog.LOG_CRON, syslog.LOG_LOCAL0, syslog.LOG_LOCAL7)

# The mask helpers compute the standard priority masks.
print("MASK(INFO):", syslog.LOG_MASK(syslog.LOG_INFO))
print("UPTO(WARNING):", syslog.LOG_UPTO(syslog.LOG_WARNING))

# openlog/syslog/closelog run without raising even with no daemon reachable, and
# setlogmask returns the previous mask and leaves it unchanged for a 0 query.
syslog.openlog("unagi-fixture", syslog.LOG_PID, syslog.LOG_USER)
syslog.syslog(syslog.LOG_INFO, "info line")
syslog.syslog("bare line")
prev = syslog.setlogmask(syslog.LOG_UPTO(syslog.LOG_ERR))
print("prev mask is int:", isinstance(prev, int))
syslog.setlogmask(prev)
syslog.closelog()
print("done")
