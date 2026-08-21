import { Link } from 'react-router-dom'

export default function LoginPage() {
  return (
    <div className="auth-page">
      <form className="auth-form">
        <h2>Log in</h2>
        <label className="field">
          Email
          <input type="email" name="email" />
        </label>
        <label className="field">
          Password
          <input type="password" name="password" />
        </label>
        <button className="btn" type="submit">
          Log in
        </button>
        <p>
          No account? <Link to="/register">Register</Link>
        </p>
      </form>
    </div>
  )
}
