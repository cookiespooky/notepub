"use client";

import { useFormState } from "react-dom";
import { updateEmailAction, updatePasswordAction, type FormState } from "./actions";
import styles from "./account.module.css";

const initialState: FormState = {};

export function EmailForm({ currentEmail }: { currentEmail: string }) {
  const [state, formAction] = useFormState(updateEmailAction, initialState);

  return (
    <form action={formAction} className={styles.card}>
      <h3 style={{ marginBottom: 0 }}>Изменить email</h3>
      <p className={styles.lead}>Укажите новый адрес — мы отправим письмо для подтверждения.</p>
      <label className={styles.field}>
        Новый email
        <input name="email" type="email" defaultValue={currentEmail} placeholder="name@example.com" />
      </label>
      <label className={styles.field}>
        Текущий пароль
        <input name="currentPassword" type="password" placeholder="••••••••" />
        <span className={styles.note}>Для изменения email требуется текущий пароль.</span>
      </label>
      {state.error && <p className={styles.error}>{state.error}</p>}
      {state.success && <p className={styles.success}>{state.success}</p>}
      <div className={styles.actions}>
        <button type="submit" className={styles.primary}>
          Сохранить
        </button>
      </div>
    </form>
  );
}

export function PasswordForm() {
  const [state, formAction] = useFormState(updatePasswordAction, initialState);

  return (
    <form action={formAction} className={styles.card}>
      <h3 style={{marginBottom: 0}}>Сменить пароль</h3>
      <p className={styles.lead}>Введите текущий пароль и новый, чтобы обновить доступ.</p>
      <div className={styles.fieldGrid}>
        <label className={styles.field}>
          Текущий пароль
          <input name="currentPassword" type="password" placeholder="••••••••" />
        </label>
        <label className={styles.field}>
          Новый пароль
          <input name="newPassword" type="password" placeholder="Минимум 6 символов" />
        </label>
        <label className={styles.field}>
          Подтверждение
          <input name="confirmPassword" type="password" placeholder="Повторите новый пароль" />
        </label>
      </div>
      {state.error && <p className={styles.error}>{state.error}</p>}
      {state.success && <p className={styles.success}>{state.success}</p>}
      <div className={styles.actions}>
        <button type="submit" className={styles.primary}>
          Обновить пароль
        </button>
      </div>
    </form>
  );
}
