package zkmul

import (
	"crypto/sha512"
	"math/big"

	"github.com/taurusgroup/cmp-ecdsa/pkg/arith"
	"github.com/taurusgroup/cmp-ecdsa/pkg/curve"
	"github.com/taurusgroup/cmp-ecdsa/pkg/paillier"
)

const domain = "CMP-MUL"

type Commitment struct {
	// A = Y^alpha r^N
	// B = Enc(alpha; s)
	A, B *paillier.Ciphertext
}

type Response struct {
	// Z = alpha + ex // TODO is this mod N as well?
	// U = r • rho^e (mod N)
	// V = s • rhoX^e (mod N)
	Z, U, V *big.Int
}

type Proof struct {
	*Commitment
	*Response
}

func (commitment *Commitment) Challenge() *big.Int {
	var e big.Int
	h := sha512.New()
	h.Write([]byte(domain))

	// TODO Which parameters should we include?
	// Write public parameters to hash
	h.Write([]byte(""))

	// Write commitments
	h.Write(commitment.A.Bytes())
	h.Write(commitment.B.Bytes())

	out := h.Sum(nil)
	e.SetBytes(out)
	e.Mod(&e, curve.Q)
	e.Sub(&e, curve.QHalf)
	return curve.Q
}

// NewProof generates a proof that the
func NewProof(verifier *paillier.PublicKey, X, Y, C *paillier.Ciphertext,
	x *big.Int, rho, rhoX *big.Int) *Proof {
	N := verifier.N()

	alpha := arith.RandomUnit(N)
	r := arith.RandomUnit(N)
	s := arith.RandomUnit(N)

	var A, B paillier.Ciphertext

	A.Mul(verifier, Y, alpha)
	A.Randomize(verifier, r)

	B.Enc(verifier, alpha, s)

	commitment := &Commitment{
		A: &A,
		B: &B,
	}

	e := commitment.Challenge()

	var z, u, v big.Int
	z.Mul(e, x)
	z.Add(&z, alpha)

	u.Exp(rho, e, N)
	u.Mul(&u, r)

	v.Exp(rhoX, e, N)
	v.Mul(&v, s)
	v.Mod(&v, N)

	response := &Response{
		Z: &z,
		U: &u,
		V: &v,
	}

	return &Proof{
		Commitment: commitment,
		Response:   response,
	}
}

func (proof *Proof) Verify(prover *paillier.PublicKey, X, Y, C *paillier.Ciphertext) bool {
	e := proof.Challenge()

	var lhs, rhs paillier.Ciphertext

	{
		// lhs = Y^z u^N
		lhs.Mul(prover, Y, proof.Z)
		lhs.Randomize(prover, proof.U)

		// rhs = A C^e
		rhs.Mul(prover, C, e)
		rhs.Add(prover, &rhs, proof.A)

		if !lhs.Equal(&rhs) {
			return false
		}
	}

	{
		// lhs = Enc(z; v)
		lhs.Enc(prover, proof.Z, proof.V)

		// rhs = B X^e
		rhs.Mul(prover, X, e)
		rhs.Add(prover, &rhs, proof.B)
		if !lhs.Equal(&rhs) {
			return false
		}
	}

	return true
}