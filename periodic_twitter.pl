#!/usr/bin/perl
use common::sense;

my $cached_status = "";

while (1) {
    # -s: silent
    my $new_status = qx(curl -s https://deresuteborder.mon.moe/twitter);
    if ($new_status =~ /^\s*$/ or $new_status =~ /UPDATING/) {
        next;
    }
    if ($new_status ne $cached_status) {
        print "update status: $new_status\n";
        system qw(perl twitter.pl), $new_status;
        $cached_status = $new_status;
    }
    sleep 120;
}
