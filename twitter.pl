#!/usr/bin/perl
# usage: perl twitter.pl "status"  # utf8 encoded
use common::sense;
use Net::Twitter;
use YAML qw(LoadFile);
use Encode qw(decode encode);
use utf8;

my $config = LoadFile("secret.yaml");
#print "$config\n";
my $nt = Net::Twitter->new(
    "traits"   => ["API::RESTv1_1",],
    "consumer_key"        => $$config{"twitter_consumer_key"},
    "consumer_secret"     => $$config{"twitter_consumer_secret"},
    "access_token"        => $$config{"twitter_access_token"},
    "access_token_secret" => $$config{"twitter_access_token_secret"},
);

my $status = "test11";

if (@ARGV) {
    $status = decode("UTF-8", pop(@ARGV));
}

eval {
    my $result = $nt->update({ status => $status });
    print "here", $result,"\n";
};

if ($@) {
    print "err: $@\n";
}
