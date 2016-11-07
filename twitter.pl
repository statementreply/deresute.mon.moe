#!/usr/bin/perl
# usage: perl twitter.pl "status"  # utf8 encoded
use common::sense;
use Net::Twitter;
use YAML qw(LoadFile);
use Encode qw(decode encode);
use utf8;

my $config = LoadFile("secret.yaml");
#print "$config\n";
if ((exists $$config{"twitter_dummy"}) and ($$config{"twitter_dummy"} != 0)) {
    # testmode
    print "twitter.pl called: <@ARGV>\n";
    #exit 0;
    # test retry
    exit 1;
}
my $nt = Net::Twitter->new(
    "ssl"      => 1,
    "traits"   => ["API::RESTv1_1",],
    "consumer_key"        => $$config{"twitter_consumer_key"},
    "consumer_secret"     => $$config{"twitter_consumer_secret"},
    "access_token"        => $$config{"twitter_access_token"},
    "access_token_secret" => $$config{"twitter_access_token_secret"},
);

my $status = "（テスト）";

if (@ARGV) {
    $status = decode("UTF-8", pop(@ARGV));
}
print "status is ", encode("UTF-8", $status);

my $result;
eval {
    $result = $nt->update({ status => $status });
    #print "here", $result,"\n";
};

if ($@) {
    my $err = $@;
    print "err: $err\n";
    die $err unless blessed $err && $err->isa("Net::Twitter::Error");
    print "err msg <", $err->error, ">\n";
    exit 1;
} else {
    print "no err: $result\n";
    exit 0;
}

