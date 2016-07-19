#!/usr/bin/perl
use common::sense;
use Net::Twitter;
use YAML qw(LoadFile);

my $config = LoadFile("secret.yaml");
#print "$config\n";
my $nt = Net::Twitter->new(
    "traits"   => ["API::RESTv1_1",],
    "consumer_key"        => $$config{"twitter_consumer_key"},
    "consumer_secret"     => $$config{"twitter_consumer_secret"},
    "access_token"        => $$config{"twitter_access_token"},
    "access_token_secret" => $$config{"twitter_access_token_secret"},
);

eval {
    my $result = $nt->update({ status => "test10"});
    print "here", $result,"\n";
};

if ($@) {
    print "err: $@\n";
}

