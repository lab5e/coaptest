# CoAP Test

This is a test bench to investigate an issue in the [go-coap](https://github.com/plgd-dev/go-coap) library when it comes to blockwise transfer.  

## Problem formulation

When you use the `udp.WithBlockwise()` option with `udp.NewServer()`, it will configure a middleware layer that deals with blockwise transfers automatically for you.  However this has the limitation that it requires the request token to remain constant. *This is not the correct behavior*.

[rfc7959](https://datatracker.ietf.org/doc/html/rfc7959) explicitly specifies
that:

> As a general comment on tokens, there is no other mention of tokens in this
> document, as block-wise transfers handle tokens like any other CoAP exchange.
> As usual, the client is free to choose tokens for each exchange as it likes.

*This is mentioned in [section 3.4](https://datatracker.ietf.org/doc/html/rfc7959#section-3.4)*

In the introduction of `RFC7959` blockwise transfer is described as

> Block-wise transfers are realized as combinations of exchanges, each of which
> is performed according to the CoAP base protocol.

This means that the `token` field is allowed to change for each exchange, thought it is not reuquired to.  In fact, it is reasonable to do so. The `go-coap` library blockwise transfer middleware uses a combination of the *token* and the *remote address* of the client to correlate exchanges that belong to a block transfer.

Which means that when a client rotates the token (*as is well within the spec*), the library will create a new blockwise transfer session for **every single exchange** and then send the next block as requested. Leading to an unholy memory explosion for anyting that is larger than a few thousand bytes.

## Interlude: RFC9177

To deal with the challenges in blockwise transfer there is [RFC9177](https://datatracker.ietf.org/doc/html/rfc9177).  This is, of course, laudable, but it does very little to mitigate the damage since there are already lots of implementations in the wild that will not be changed in the foreseeable future, so we have to make the best of what we have.

## Demonstrating the problem

This project produces two binaries:

- `coapd` - a simple server that provides a `/fw` endpoint that returns an image
- `block` - a simple client that can be used to fetch a resource

Start the server in one window:

```shell
bin/coapd
```

Run the client **without** token rotation like this:

```shell
bin/block coap://localhost:5683/fw 
```

Run the client **with** token rotation like this:

```shell
bin/block coap://localhost:5683/fw --rotate
```

Observe the difference in output from the `coapd` process.  When you rotate the tokens for each exchange (which is valid behavior), the `fwHandler` creates a new blockwise transfer *for every single block* that the client requests.  When you do not rotate the tokens it behaves.

## Solution

The way to solve this is probably to provide a new middleware implementation for the blockwise transfer and avoid the one that comes with the library.  It doesn't seem like the maintainers have quite figured out what to do yet, so we'll have to have a crack at it.

It is possible that we can use the `udp.WithOnNewClientConn()` option and hook into the context, but I fear there may be dragons.

Check back later for more.  I'm looking into a workaround.
