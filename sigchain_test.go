package keys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSigchain(t *testing.T) {
	clock := newClock()
	alice := NewEd25519KeyFromSeed(Bytes32(bytes.Repeat([]byte{0x01}, 32)))

	sc := NewSigchain(alice.PublicKey())
	require.Equal(t, 0, sc.Length())

	st, err := GenerateStatement(sc, bytes.Repeat([]byte{0x01}, 16), alice, "test", clock.Now())
	require.NoError(t, err)
	err = sc.Add(st)
	require.NoError(t, err)
	require.Equal(t, 1, sc.LastSeq())

	res := sc.FindLast("test")
	require.NotNil(t, res)
	require.Equal(t, bytes.Repeat([]byte{0x01}, 16), res.Data)
	require.Equal(t, 1, sc.Length())

	_, err = sc.Revoke(1, alice)
	require.NoError(t, err)
	require.True(t, sc.IsRevoked(1))
	require.Equal(t, 2, sc.Length())
	require.Equal(t, 2, sc.LastSeq())

	res = sc.FindLast("test")
	require.Nil(t, res)

	st2, err := GenerateStatement(sc, bytes.Repeat([]byte{0x02}, 16), alice, "test", clock.Now())
	require.NoError(t, err)
	siErr2 := sc.Add(st2)
	require.NoError(t, siErr2)

	res = sc.FindLast("test")
	require.NotNil(t, res)
	require.Equal(t, bytes.Repeat([]byte{0x02}, 16), res.Data)

	st3, err := GenerateStatement(sc, bytes.Repeat([]byte{0x03}, 16), alice, "test", clock.Now())
	require.NoError(t, err)
	siErr3 := sc.Add(st3)
	require.NoError(t, siErr3)

	res = sc.FindLast("")
	require.NotNil(t, res)
	require.Equal(t, bytes.Repeat([]byte{0x03}, 16), res.Data)

	sts := sc.FindAll("test")
	require.Equal(t, 2, len(sts))

	require.Equal(t, 4, len(sc.Statements()))

	st4, err := GenerateStatement(sc, []byte{}, alice, "", clock.Now())
	require.NoError(t, err)
	err = sc.Add(st4)
	require.EqualError(t, err, "no data")

	_, err = GenerateStatement(sc, []byte{}, GenerateEd25519Key(), "", clock.Now())
	require.EqualError(t, err, "invalid sigchain sign public key")

	// Revoke invalid seq
	_, err = sc.Revoke(0, alice)
	require.EqualError(t, err, "invalid revoke seq 0")

	// Revoke statement that doesn't exist
	_, err = sc.Revoke(10000, alice)
	require.EqualError(t, err, "invalid revoke seq 10000")

	// Revoke again
	_, err = sc.Revoke(1, alice)
	require.EqualError(t, err, "already revoked")

	// Revoke self
	_, err = sc.Revoke(5, alice)
	require.EqualError(t, err, "invalid revoke seq 5")

	spew, err := sc.Spew()
	require.NoError(t, err)
	expected, err := ioutil.ReadFile("testdata/sc2.spew")
	require.NoError(t, err)
	require.Equal(t, string(expected), spew.String())
}

func TestSigchainJSON(t *testing.T) {
	clock := newClock()
	sk := NewEd25519KeyFromSeed(Bytes32(bytes.Repeat([]byte{0x01}, 32)))

	sc := NewSigchain(sk.PublicKey())

	st, err := GenerateStatement(sc, bytes.Repeat([]byte{0x01}, 16), sk, "", clock.Now())
	require.NoError(t, err)
	err = sc.Add(st)
	require.NoError(t, err)

	st0 := sc.Statements()[0]
	expectedStatement := `{".sig":"SPKxMlhPU7wiPGsszrQN3ljWdkTbKFWxqbTqtoFp/ZrV0jd1WsMxMltiyHc4/N0mUWga1zshztXQFkEcamvECg==","data":"AQEBAQEBAQEBAQEBAQEBAQ==","kid":"kse132yw8ht5p8cetl2jmvknewjawt9xwzdlrk2pyxlnwjyqrdq0dawquwc7vw","seq":1,"ts":1234567890001}`
	require.Equal(t, expectedStatement, string(st0.Bytes()))

	b, err := json.Marshal(st0)
	require.NoError(t, err)
	require.Equal(t, expectedStatement, string(b))

	stb, err := StatementFromBytes(b)
	require.NoError(t, err)
	bout := stb.Bytes()
	require.Equal(t, expectedStatement, string(bout))

	st2, err := GenerateStatement(sc, bytes.Repeat([]byte{0x02}, 16), sk, "", clock.Now())
	require.NoError(t, err)
	siErr2 := sc.Add(st2)
	require.NoError(t, siErr2)
	entry2 := sc.Statements()[1]
	expectedStatement2 := `{".sig":"97dCpuu8cXBnMDsbsdljBAdSVV6FaWyx+Nwvw7tsk1Riksy0k5rg8OJiN0RNXPcXlHHagPku9SIlAvgQtjLpCw==","data":"AgICAgICAgICAgICAgICAg==","kid":"kse132yw8ht5p8cetl2jmvknewjawt9xwzdlrk2pyxlnwjyqrdq0dawquwc7vw","prev":"xsF9vVfMVzvoYUmrcMvWRNYpXaTrbINMgVQRHUBRQOQ=","seq":2,"ts":1234567890002}`
	require.Equal(t, expectedStatement2, string(entry2.Bytes()))

	_, siErr3 := sc.Revoke(2, sk)
	require.NoError(t, siErr3)
	entry3 := sc.Statements()[2]
	expectedStatement3 := `{".sig":"odu1EYdLq8LvKAaW80Kfoil+tdPIsvug2psWmk8Xk/UTAyczw/g5PyyKypPQaJg1/sls/qGunoTY7qcKjEgZAw==","kid":"kse132yw8ht5p8cetl2jmvknewjawt9xwzdlrk2pyxlnwjyqrdq0dawquwc7vw","prev":"txNhm/TGe8QKScMetXrv2UzDYBZ7ZI6u0TJDdoB9Cb0=","revoke":2,"seq":3,"type":"revoke"}`
	require.Equal(t, expectedStatement3, string(entry3.Bytes()))
}

func TestSigchainUsers(t *testing.T) {
	clock := newClock()
	req := NewMockRequestor()
	dst := NewMem()
	scs := NewSigchainStore(dst)
	ust := testUserStore(t, dst, scs, req, clock)
	alice := NewEd25519KeyFromSeed(Bytes32(bytes.Repeat([]byte{0x01}, 32)))

	sc := NewSigchain(alice.PublicKey())
	require.Equal(t, 0, sc.Length())

	user, err := sc.User()
	require.NoError(t, err)
	require.Nil(t, user)

	user, err = NewUser(ust, alice.ID(), "github", "alice", "https://gist.github.com/alice/70281cc427850c272a8574af4d8564d9", sc.LastSeq()+1)
	require.NoError(t, err)
	st, err := GenerateUserStatement(sc, user, alice, clock.Now())
	require.NoError(t, err)
	err = sc.Add(st)
	require.NoError(t, err)

	user, err = sc.User()
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, "alice", user.Name)
	require.Equal(t, "github", user.Service)
	require.Equal(t, "https://gist.github.com/alice/70281cc427850c272a8574af4d8564d9", user.URL)
	require.Equal(t, 1, user.Seq)

	_, err = sc.Revoke(1, alice)
	require.NoError(t, err)
	user, err = sc.User()
	require.NoError(t, err)
	require.Nil(t, user)

	user2, err := NewUser(ust, alice.ID(), "github", "alice", "https://gist.github.com/alice/a7b1370270e2672d4ae88fa5d0c6ade7", 1)
	require.NoError(t, err)
	st2, err := GenerateUserStatement(sc, user2, alice, clock.Now())
	require.EqualError(t, err, "user seq mismatch")

	user2, err = NewUser(ust, alice.ID(), "github", "alice", "https://gist.github.com/alice/a7b1370270e2672d4ae88fa5d0c6ade7", 3)
	require.NoError(t, err)
	st2, err = GenerateUserStatement(sc, user2, alice, clock.Now())
	require.NoError(t, err)
	err = sc.Add(st2)
	require.NoError(t, err)

	user, err = sc.User()
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, "alice", user.Name)
	require.Equal(t, "github", user.Service)
	require.Equal(t, "https://gist.github.com/alice/a7b1370270e2672d4ae88fa5d0c6ade7", user.URL)
	require.Equal(t, 3, user.Seq)
}

func ExampleNewSigchain() {
	clock := newClock()
	alice := GenerateEd25519Key()
	sc := NewSigchain(alice.PublicKey())

	// Create root statement
	st, err := GenerateStatement(sc, []byte("hi! 🤓"), alice, "", clock.Now())
	if err != nil {
		log.Fatal(err)
	}
	if err := sc.Add(st); err != nil {
		log.Fatal(err)
	}

	// Add 2nd statement
	st2, err := GenerateStatement(sc, []byte("2nd message"), alice, "", clock.Now())
	if err != nil {
		log.Fatal(err)
	}
	if err := sc.Add(st2); err != nil {
		log.Fatal(err)
	}

	// Revoke 2nd statement
	_, err = sc.Revoke(2, alice)
	if err != nil {
		log.Fatal(err)
	}

	// Spew
	spew, err := sc.Spew()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(spew.String())
}
