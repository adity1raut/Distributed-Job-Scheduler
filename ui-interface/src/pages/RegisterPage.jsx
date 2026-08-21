import { Link } from 'react-router-dom'

export default function RegisterPage() {
  return (
    <div className="auth-page">
      <form className="auth-form">
        <h2>Create account</h2>
        <label className="field">
          Email
          <input type="email" name="email" />
        </label>
        <label className="field">
          Password
          <input type="password" name="password" />
        </label>
        <button className="btn" type="submit">
          Create account
        </button>
        <p>
          Already have an account? <Link to="/login">Log in</Link>
        </p>
      </form>
    </div>
  )
}
